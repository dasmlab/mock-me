package cheapcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dasmlab/mock-me/internal/mockup"
)

// DefaultURL is the prod-1 OCP Route (no HAProxy basic auth).
const DefaultURL = "https://cheapcloud-dasmlab.apps.2026-prod-1.ocp.dasmlab.org"

// Client talks to cheapcloud COST-ME + import/track.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewFromEnv() *Client {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("CHEAPCLOUD_URL")), "/")
	if base == "" {
		base = DefaultURL
	}
	return &Client{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

// Target mirrors cheapcloud CostMeTarget.
type Target struct {
	Capability   string  `json:"capability"`
	Provider     string  `json:"provider,omitempty"`
	Count        int     `json:"count,omitempty"`
	Spot         *bool   `json:"spot,omitempty"`
	RegionHint   string  `json:"region_hint,omitempty"`
	SKUHint      string  `json:"sku_hint,omitempty"`
	StorageGBEst float64 `json:"storage_gb_est,omitempty"`
}

// Request is POST /api/v1/cost-me body.
type Request struct {
	ProductID         string   `json:"product_id,omitempty"`
	MockupID          string   `json:"mockup_id,omitempty"`
	RegisterFootprint bool     `json:"register_footprint,omitempty"`
	Targets           []Target `json:"targets"`
}

// Report is the cheapcloud response (opaque map + typed convenience fields).
type Report map[string]any

// ImportRequest is POST /api/v1/import/mockup.
type ImportRequest struct {
	ProductID    string           `json:"product_id,omitempty"`
	DisplayName  string           `json:"display_name,omitempty"`
	MockupID     string           `json:"mockup_id,omitempty"`
	Envelope     string           `json:"envelope,omitempty"`
	Notes        string           `json:"notes,omitempty"`
	AttachPolicy string           `json:"attach_policy,omitempty"`
	Components   []map[string]any `json:"components,omitempty"`
}

// ImportResult is the import response.
type ImportResult map[string]any

// TrackedResponse is GET /api/v1/home/tracked.
type TrackedResponse struct {
	Objects []map[string]any `json:"objects"`
	Summary map[string]any   `json:"summary"`
}

// CostMe calls cheapcloud and returns the report JSON.
func (c *Client) CostMe(req Request) (Report, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("cheapcloud client not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + "/api/v1/cost-me"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cheapcloud cost-me: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cheapcloud cost-me %s: %s", res.Status, truncate(string(raw), 300))
	}
	var report Report
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decode cost-me: %w", err)
	}
	return report, nil
}

// ImportMockUp registers a MockUp as a tracked cheapcloud footprint.
func (c *Client) ImportMockUp(req ImportRequest) (ImportResult, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("cheapcloud client not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/v1/import/mockup", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cheapcloud import: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cheapcloud import %s: %s", res.Status, truncate(string(raw), 300))
	}
	var out ImportResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode import: %w", err)
	}
	return out, nil
}

// TrackedByMockUp fetches Home tracked objects for a MockUp id.
func (c *Client) TrackedByMockUp(mockupID string) (TrackedResponse, error) {
	var out TrackedResponse
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("cheapcloud client not configured")
	}
	u := c.BaseURL + "/api/v1/home/tracked?mockup_id=" + url.QueryEscape(mockupID)
	httpReq, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	httpReq.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("cheapcloud tracked: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return out, fmt.Errorf("cheapcloud tracked %s: %s", res.Status, truncate(string(raw), 300))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode tracked: %w", err)
	}
	return out, nil
}

// TargetsFromMockUp maps style + Model cloud orphans → COST-ME capability targets.
func TargetsFromMockUp(m *mockup.MockUp) []Target {
	if m == nil {
		return nil
	}
	spot := true
	// Prefer explicit Model cloud blocks when present.
	if cloud := targetsFromCloudModel(m); len(cloud) > 0 {
		return cloud
	}
	style := m.Spec.Style
	switch style {
	case mockup.StyleAROAzureLab:
		return targetsFromAROLab(m)
	case mockup.StyleSingleSNOOCP:
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: 1, Spot: &spot,
		}}
	case mockup.StyleACMMultiCluster, "":
		n := 1 + len(m.Spec.Clusters) // hub SNO + deployments
		if n < 1 {
			n = 1
		}
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: n, Spot: &spot,
		}}
	case mockup.StyleSurfingCdnR2, mockup.StyleSelfServePersonalCDN:
		return []Target{{
			Capability: "object-store", Provider: "r2", StorageGBEst: 8,
		}}
	case mockup.StyleCloudCostModel:
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: 1, Spot: &spot,
		}}
	default:
		return []Target{{
			Capability: "ocp-sno-slim", Provider: "azure", Count: 1, Spot: &spot,
		}}
	}
}

func targetsFromAROLab(m *mockup.MockUp) []Target {
	// Prefer canvas orphans when present (baseline + optional Spot).
	spotCount := 0
	workerSKU := "Standard_D4s_v3"
	region := "eastus"
	if m.Spec.Canvas != nil {
		for _, n := range m.Spec.Canvas.Orphans {
			switch n.Kind {
			case "cloud-aro-spot-worker":
				spotCount += noteCount(n.Notes, 2)
				if s := noteKV(n.Notes, "sku"); s != "" {
					workerSKU = s
				}
			case "cloud-aro-worker":
				if s := noteKV(n.Notes, "sku"); s != "" {
					workerSKU = s
				}
			case "cloud-aro-cluster":
				if r := noteKV(n.Notes, "region"); r != "" {
					region = r
				}
			}
		}
	}
	wantSpot := spotCount > 0
	t := Target{
		Capability: "aro-minimal",
		Provider:   "azure",
		RegionHint: region,
		SKUHint:    workerSKU,
		Spot:       &wantSpot,
	}
	if wantSpot {
		t.Count = spotCount
	}
	return []Target{t}
}

func noteCount(notes string, def int) int {
	if v := noteKV(notes, "count"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func noteKV(notes, key string) string {
	keyPref := strings.ToLower(key) + "="
	for _, part := range strings.Fields(notes) {
		if strings.HasPrefix(strings.ToLower(part), keyPref) {
			// preserve original value casing (Azure SKUs)
			idx := strings.Index(strings.ToLower(part), "=")
			if idx >= 0 && idx+1 < len(part) {
				return part[idx+1:]
			}
		}
	}
	return ""
}

func targetsFromCloudModel(m *mockup.MockUp) []Target {
	if m.Spec.Canvas == nil {
		return nil
	}
	spot := true
	var out []Target
	var snoAzure, snoAWS, snoGCP, vmAzure, vmAWS, vmGCP, r2 int
	for _, n := range m.Spec.Canvas.Orphans {
		switch n.Kind {
		case "cloud-ocp-sno-slim":
			// Notes may hint provider=aws|gcp
			switch providerHint(n.Notes) {
			case "aws":
				snoAWS++
			case "gcp":
				snoGCP++
			default:
				snoAzure++
			}
		case "cloud-vm-spot":
			vmAzure++
		case "cloud-aws-ec2-spot":
			vmAWS++
		case "cloud-gcp-gce-spot":
			vmGCP++
		case "cloud-r2", "cloud-object-store":
			r2++
		}
	}
	if snoAzure > 0 {
		out = append(out, Target{Capability: "ocp-sno-slim", Provider: "azure", Count: snoAzure, Spot: &spot})
	}
	if snoAWS > 0 {
		out = append(out, Target{Capability: "ocp-sno-slim", Provider: "aws", Count: snoAWS, Spot: &spot, SKUHint: "m5.2xlarge"})
	}
	if snoGCP > 0 {
		out = append(out, Target{Capability: "ocp-sno-slim", Provider: "gcp", Count: snoGCP, Spot: &spot, SKUHint: "n2-standard-8"})
	}
	if vmAzure > 0 {
		out = append(out, Target{Capability: "azure-spot-vm", Provider: "azure", Count: vmAzure, Spot: &spot})
	}
	if vmAWS > 0 {
		out = append(out, Target{Capability: "aws-spot-vm", Provider: "aws", Count: vmAWS, Spot: &spot, SKUHint: "m5.xlarge"})
	}
	if vmGCP > 0 {
		out = append(out, Target{Capability: "gcp-spot-vm", Provider: "gcp", Count: vmGCP, Spot: &spot, SKUHint: "n2-standard-4"})
	}
	if r2 > 0 {
		out = append(out, Target{Capability: "object-store", Provider: "r2", StorageGBEst: float64(8 * r2)})
	}
	return out
}

func providerHint(notes string) string {
	n := strings.ToLower(notes)
	if strings.Contains(n, "provider=aws") {
		return "aws"
	}
	if strings.Contains(n, "provider=gcp") {
		return "gcp"
	}
	return ""
}

// ProductID is the canonical cheapcloud product id for a MockUp: mock-me-<uuid>.
func ProductID(m *mockup.MockUp) string {
	if m == nil {
		return "mock-me"
	}
	if pid := strings.TrimSpace(m.Status.CheapcloudProductID); pid != "" {
		return pid
	}
	id := strings.TrimSpace(m.Metadata.ID)
	if id == "" {
		return "mock-me"
	}
	return "mock-me-" + id
}

// ImportBody builds an ImportRequest from a MockUp.
func ImportBody(m *mockup.MockUp, attachPolicy string) ImportRequest {
	pid := ProductID(m)
	targets := TargetsFromMockUp(m)
	env := envelopeFromTargets(targets)
	comps := componentsFromTargets(targets)
	return ImportRequest{
		ProductID:    pid,
		DisplayName:  firstNonEmpty(m.Metadata.Name, pid),
		MockupID:     m.Metadata.ID,
		Envelope:     env,
		AttachPolicy: attachPolicy,
		Notes:        "imported from mock-me Model",
		Components:   comps,
	}
}

func envelopeFromTargets(targets []Target) string {
	counts := map[string]int{}
	for _, t := range targets {
		switch strings.ToLower(t.Provider) {
		case "aws":
			counts["aws-compute"]++
		case "gcp":
			counts["gcp-compute"]++
		case "r2", "cloudflare":
			counts["surfing-cdn-storage"]++
		default:
			counts["azure-compute"]++
		}
	}
	best, n := "azure-compute", 0
	for env, c := range counts {
		if c > n {
			best, n = env, c
		}
	}
	return best
}

func componentsFromTargets(targets []Target) []map[string]any {
	out := make([]map[string]any, 0, len(targets))
	for i, t := range targets {
		kind := "compute"
		if t.Capability == "object-store" {
			kind = "storage"
		}
		prov := t.Provider
		if prov == "" {
			prov = "azure"
		}
		out = append(out, map[string]any{
			"id":       fmt.Sprintf("%s-%d", t.Capability, i),
			"kind":     kind,
			"label":    t.Capability,
			"provider": prov,
			"opex":     "metered",
			"notes":    fmt.Sprintf("from MockUp model · count=%d", max(1, t.Count)),
		})
	}
	if len(out) == 0 {
		out = append(out, map[string]any{
			"id": "imported", "kind": "compute", "label": "Imported from mock-me",
			"provider": "azure", "opex": "metered",
		})
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
