// Package demo implements the shared Demo / unknown visitor facade for mock-me.
// See dasmlab_home/docs/DEMO-VISITOR-CONTRACT.md
package demo

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	CookieName = "mm_demo"
	CookieTTL  = 2 * time.Hour
)

// Job mirrors deploy.Job JSON shape for the assembly-line UI (fixtures only).
type Stage struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
	Icon    string `json:"icon"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Log     string `json:"log,omitempty"`
}

type ConsoleLine struct {
	At    string `json:"at"`
	Stage string `json:"stage,omitempty"`
	Text  string `json:"text"`
}

type Job struct {
	ID           string        `json:"id"`
	MockUpID     string        `json:"mockUpId"`
	InventoryID  string        `json:"inventoryId"`
	HostName     string        `json:"hostName,omitempty"`
	HostEndpoint string        `json:"hostEndpoint,omitempty"`
	Status       string        `json:"status"`
	Message      string        `json:"message,omitempty"`
	Stages       []Stage       `json:"stages"`
	Console      []ConsoleLine `json:"console,omitempty"`
	StartedAt    string        `json:"startedAt"`
	UpdatedAt    string        `json:"updatedAt"`
	FinishedAt   string        `json:"finishedAt,omitempty"`
	Demo         bool          `json:"demo"`
}

var (
	mu   sync.Mutex
	jobs = map[string]*Job{} // keyed by demo session cookie value
)

func IsDemo(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	return err == nil && c.Value != ""
}

func SessionID(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func SetCookie(w http.ResponseWriter, r *http.Request, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(CookieTTL.Seconds()),
	})
}

func ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Enter creates a demo session cookie (no Keycloak).
func Enter(w http.ResponseWriter, r *http.Request) {
	sid := uuid.New().String()
	SetCookie(w, r, sid)
	writeJSON(w, http.StatusOK, map[string]any{
		"demo":    true,
		"session": sid,
		"notice":  "Demo / fake mode — not a live system. No live node deploys.",
	})
}

func Exit(w http.ResponseWriter, r *http.Request) {
	if sid := SessionID(r); sid != "" {
		mu.Lock()
		delete(jobs, sid)
		mu.Unlock()
	}
	ClearCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"demo": false})
}

func Me(w http.ResponseWriter, r *http.Request) {
	if !IsDemo(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no demo session"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"preferred_username": "demo-visitor",
		"name":               "Demo Visitor",
		"demo":               true,
		"is_admin":           false,
	})
}

// Simulate returns a scripted deploy timeline — never calls live deploy paths.
func Simulate(w http.ResponseWriter, r *http.Request) {
	if !IsDemo(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "demo session required"})
		return
	}
	sid := SessionID(r)
	now := time.Now().UTC()
	job := &Job{
		ID:           uuid.New().String(),
		MockUpID:     "demo-mockup",
		InventoryID:  "demo-inventory",
		HostName:     "demo-host (fake)",
		HostEndpoint: "127.0.0.1 (simulated)",
		Status:       "succeeded",
		Message:      "Simulated deploy complete — no live systems were touched.",
		Demo:         true,
		StartedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Add(12 * time.Second).Format(time.RFC3339),
		FinishedAt:   now.Add(12 * time.Second).Format(time.RFC3339),
		Stages: []Stage{
			{ID: "generate", Label: "Generate", Detail: "Render mockup artifacts", Icon: "description", Status: "ok", Message: "Fixtures rendered"},
			{ID: "ee", Label: "Execution Env", Detail: "Prepare EE", Icon: "terminal", Status: "ok", Message: "Simulated EE ready"},
			{ID: "vinfra", Label: "Virtual Infra", Detail: "VMs / networking", Icon: "dns", Status: "ok", Message: "Fake VMs allocated"},
			{ID: "ocp", Label: "OpenShift", Detail: "Cluster install", Icon: "hub", Status: "ok", Message: "Simulated OCP up"},
			{ID: "acm", Label: "ACM", Detail: "Hub registration", Icon: "account_tree", Status: "ok", Message: "Simulated ACM linked"},
			{ID: "spokes", Label: "Spokes", Detail: "Managed clusters", Icon: "device_hub", Status: "ok", Message: "Simulated spokes online"},
		},
		Console: []ConsoleLine{
			{At: now.Format(time.RFC3339), Stage: "generate", Text: "[demo] generate fixtures (no disk writes to prod)"},
			{At: now.Add(2 * time.Second).Format(time.RFC3339), Stage: "ee", Text: "[demo] skip live ansible / EE"},
			{At: now.Add(4 * time.Second).Format(time.RFC3339), Stage: "vinfra", Text: "[demo] skip hypervisor provisioning"},
			{At: now.Add(6 * time.Second).Format(time.RFC3339), Stage: "ocp", Text: "[demo] skip openshift-install"},
			{At: now.Add(8 * time.Second).Format(time.RFC3339), Stage: "acm", Text: "[demo] skip ACM import"},
			{At: now.Add(10 * time.Second).Format(time.RFC3339), Stage: "spokes", Text: "[demo] skip spoke join — done"},
		},
	}
	mu.Lock()
	jobs[sid] = job
	mu.Unlock()
	writeJSON(w, http.StatusOK, job)
}

func Status(w http.ResponseWriter, r *http.Request) {
	if !IsDemo(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "demo session required"})
		return
	}
	sid := SessionID(r)
	mu.Lock()
	job := jobs[sid]
	mu.Unlock()
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no simulated job yet — POST /demo/simulate"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// DenyMutate rejects mutate methods when only a demo cookie is present (no admin session).
func DenyMutate(isAdmin func(http.ResponseWriter, *http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if IsDemo(r) && (isAdmin == nil || !isAdmin(w, r)) {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "demo mode cannot mutate live systems — use /api/v1/demo/simulate",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
