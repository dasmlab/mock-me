// Package mockup stores product canvases (MockUp). Genre + style select the offering
// (e.g. cluster-management / acm-multi-cluster); ACM lab rack is the first style.
package mockup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Phase tracks MockUp lifecycle (list badge + validate/deploy click-through).
type Phase string

const (
	PhaseCreated    Phase = "created"
	PhaseConfigured Phase = "configured" // derived YAML under out/
	PhaseValidated  Phase = "validated"  // topology (+ plan) checks passed
	PhaseDeploying  Phase = "deploying"  // deploy job running against inventory
	PhaseDeployed   Phase = "deployed"   // plan accepted / orchestration complete (MVP)
	PhaseFailed     Phase = "failed"     // deploy stopped — Clean or Delete before continuing
	// Legacy milestones retained for older mockups / future fine-grained progress.
	PhaseHubReady  Phase = "hub-ready"
	PhaseACMReady  Phase = "acm-ready"
	PhaseClustered Phase = "clustered"
	PhaseReady     Phase = "ready"
)

// MockUp is the top-level lab rack object (like a Target in etcd-synthetic-load).
type MockUp struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Spec       Spec     `json:"spec" yaml:"spec"`
	Status     Status   `json:"status" yaml:"status"`
	Layout     Layout   `json:"layout" yaml:"layout"`
}

type Metadata struct {
	ID        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`
	Notes     string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type Spec struct {
	// Genre + Style select the product offering (catalog). Default: cluster-management / acm-multi-cluster.
	Genre string `json:"genre,omitempty" yaml:"genre,omitempty"`
	Style string `json:"style,omitempty" yaml:"style,omitempty"`

	BaseDomain string      `json:"baseDomain" yaml:"baseDomain"`
	Provider   string      `json:"provider" yaml:"provider"`
	Network    NetworkSpec `json:"network" yaml:"network"`
	// CanvasMode: guided (default rack, constrained relations) | freeform (teaching / creative).
	CanvasMode string `json:"canvasMode,omitempty" yaml:"canvasMode,omitempty"`
	// Canvas holds free-form teaching state (omits, orphan drops). Derive ignores orphans.
	Canvas *CanvasSpec `json:"canvas,omitempty" yaml:"canvas,omitempty"`
	// InfraHost = MACHINE HOST: RHEL (BM or nested) where libvirtd/podman run.
	InfraHost InfraHostNode `json:"infraHost" yaml:"infraHost"`
	// Gateway = VyOS (or similar) VM: WAN on host bridge, LAN = lab libvirt net.
	Gateway GatewayNode `json:"gateway" yaml:"gateway"`
	// Hub = MGMT-CLUSTER SNO guest VM (governance OCP); ACM operators on top.
	Hub      HubNode       `json:"hub" yaml:"hub"`
	ACM      ACMNode       `json:"acm" yaml:"acm"`
	Clusters []ClusterNode `json:"clusters" yaml:"clusters"`
	Gaps     GapParams     `json:"gaps" yaml:"gaps"`
}

// CanvasSpec is free-form / creative topology state for teaching demos.
// TODO(later): promote free-form → guided constrained MockUp is intentionally unsupported.
type CanvasSpec struct {
	// ShowRelations: when false (default in freeform), canvas draws no constrained edges.
	ShowRelations bool `json:"showRelations,omitempty" yaml:"showRelations,omitempty"`
	// Omit* hides rack objects from free-form view + validate (objects remain in YAML for undo).
	OmitHost    bool `json:"omitHost,omitempty" yaml:"omitHost,omitempty"`
	OmitGateway bool `json:"omitGateway,omitempty" yaml:"omitGateway,omitempty"`
	OmitHub     bool `json:"omitHub,omitempty" yaml:"omitHub,omitempty"`
	OmitACM     bool `json:"omitACM,omitempty" yaml:"omitACM,omitempty"`
	// Orphans: free-form drops (vHosts / appliances) not owned by guided derive.
	Orphans []CanvasNode `json:"orphans,omitempty" yaml:"orphans,omitempty"`
}

// CanvasNode is a free-form teaching object (vHost or appliance payload).
type CanvasNode struct {
	ID            string `json:"id" yaml:"id"`
	Kind          string `json:"kind" yaml:"kind"` // vhost | appliance
	Label         string `json:"label" yaml:"label"`
	ApplianceType string `json:"applianceType,omitempty" yaml:"applianceType,omitempty"` // vyos | haproxy | other
	// RunsOn: appliance → vHost id it sits on (middleware / payload on the guest).
	RunsOn string  `json:"runsOn,omitempty" yaml:"runsOn,omitempty"`
	Notes  string  `json:"notes,omitempty" yaml:"notes,omitempty"`
	X      float64 `json:"x,omitempty" yaml:"x,omitempty"`
	Y      float64 `json:"y,omitempty" yaml:"y,omitempty"`
}

// InfraHostNode is the MACHINE HOST (libvirtd). Not an OCP node.
// Disks: small system + large pool for guest images. NICs: bridged uplink
// (+ optional host-only/dummy for future class moves).
type InfraHostNode struct {
	ID           string     `json:"id" yaml:"id"`
	Label        string     `json:"label" yaml:"label"` // MACHINE-HOST / INFRA-HOST
	Hostname     string     `json:"hostname" yaml:"hostname"`
	Kind         string     `json:"kind" yaml:"kind"`                                 // baremetal | nested-vm
	Hypervisor   string     `json:"hypervisor,omitempty" yaml:"hypervisor,omitempty"` // vmware | kvm | none
	OS           string     `json:"os" yaml:"os"`                                     // rhel-9 | rhel-10
	Arch         string     `json:"arch,omitempty" yaml:"arch,omitempty"`
	CPU          int        `json:"cpu" yaml:"cpu"`
	MemoryMiB    int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB      int        `json:"diskGiB" yaml:"diskGiB"`
	Disks        []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs         []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	LibvirtURI   string     `json:"libvirtURI,omitempty" yaml:"libvirtURI,omitempty"`
	NetworkName  string     `json:"networkName,omitempty" yaml:"networkName,omitempty"` // lab LAN net name
	StoragePool  string     `json:"storagePool,omitempty" yaml:"storagePool,omitempty"`
	Podman       bool       `json:"podman" yaml:"podman"`
	SSHHost      string     `json:"sshHost,omitempty" yaml:"sshHost,omitempty"`
	SSHUser      string     `json:"sshUser,omitempty" yaml:"sshUser,omitempty"`
	InventoryRef string     `json:"inventoryRef,omitempty" yaml:"inventoryRef,omitempty"` // MachineHost inventory id
	Notes        string     `json:"notes,omitempty" yaml:"notes,omitempty"`
	ACMReference string     `json:"acmReference,omitempty" yaml:"acmReference,omitempty"`
}

// GatewayNode is the lab edge router VM (VyOS): NAT/FW between real bridge (WAN)
// and the obscure private libvirt LAN where hub + deployment guests live.
type GatewayNode struct {
	ID         string     `json:"id" yaml:"id"`
	Label      string     `json:"label" yaml:"label"` // VYOS-GW
	Hostname   string     `json:"hostname" yaml:"hostname"`
	Image      string     `json:"image,omitempty" yaml:"image,omitempty"` // vyos | similar
	ISOPath    string     `json:"isoPath,omitempty" yaml:"isoPath,omitempty"`
	CPU        int        `json:"cpu" yaml:"cpu"`
	MemoryMiB  int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int        `json:"diskGiB" yaml:"diskGiB"`
	Disks      []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs       []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"` // eth0 WAN, eth1 LAN
	WANBridge  string     `json:"wanBridge,omitempty" yaml:"wanBridge,omitempty"`
	LANNetwork string     `json:"lanNetwork,omitempty" yaml:"lanNetwork,omitempty"`
	LANCIDR    string     `json:"lanCIDR,omitempty" yaml:"lanCIDR,omitempty"`
	LANIP      string     `json:"lanIP,omitempty" yaml:"lanIP,omitempty"` // typically .1
	NAT        bool       `json:"nat" yaml:"nat"`
	Firewall   bool       `json:"firewall" yaml:"firewall"`
	Phase      string     `json:"phase,omitempty" yaml:"phase,omitempty"` // planned | booted | configured
	Notes      string     `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// DiskSpec describes one block device (vHOST inventory or guest VM disk).
type DiskSpec struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	SizeGiB int    `json:"sizeGiB" yaml:"sizeGiB"`
	Bus     string `json:"bus,omitempty" yaml:"bus,omitempty"`   // nvme | virtio | sata
	Role    string `json:"role,omitempty" yaml:"role,omitempty"` // system | data | pool
}

// NICSpec describes one network interface.
type NICSpec struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Model   string `json:"model,omitempty" yaml:"model,omitempty"`     // virtio | e1000e
	Mode    string `json:"mode,omitempty" yaml:"mode,omitempty"`       // bridged | nat | isolated | libvirt-network | host-only
	Network string `json:"network,omitempty" yaml:"network,omitempty"` // bridge / vmnet / libvirt net
	Role    string `json:"role,omitempty" yaml:"role,omitempty"`       // wan | lan | uplink | host-only | guest
}

type NetworkSpec struct {
	MachineCIDR string `json:"machineCIDR" yaml:"machineCIDR"`
	Gateway     string `json:"gateway" yaml:"gateway"`
	APIVIP      string `json:"apiVIP" yaml:"apiVIP"`
	IngressVIP  string `json:"ingressVIP" yaml:"ingressVIP"`
	DHCPStart   string `json:"dhcpStart,omitempty" yaml:"dhcpStart,omitempty"`
	DHCPEnd     string `json:"dhcpEnd,omitempty" yaml:"dhcpEnd,omitempty"`
	DNS         string `json:"dns,omitempty" yaml:"dns,omitempty"`
}

type HubNode struct {
	ID         string     `json:"id" yaml:"id"`
	Label      string     `json:"label" yaml:"label"` // MGMT-CLUSTER
	Mode       string     `json:"mode" yaml:"mode"`
	Version    string     `json:"version" yaml:"version"`
	Profile    string     `json:"profile" yaml:"profile"`
	Hostname   string     `json:"hostname" yaml:"hostname"`
	IP         string     `json:"ip" yaml:"ip"`
	MAC        string     `json:"mac" yaml:"mac"`
	CPU        int        `json:"cpu" yaml:"cpu"`
	MemoryMiB  int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int        `json:"diskGiB" yaml:"diskGiB"`
	Disks      []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs       []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	InstallACM bool       `json:"installACM" yaml:"installACM"`
}

type ACMNode struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"` // ACM
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	MCEChannel string `json:"mceChannel" yaml:"mceChannel"`
	ACMChannel string `json:"acmChannel" yaml:"acmChannel"`
	Notes      string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ClusterNode struct {
	ID         string     `json:"id" yaml:"id"`
	Label      string     `json:"label" yaml:"label"` // DEPLOYMENT-CLUSTER-X
	Name       string     `json:"name" yaml:"name"`
	Version    string     `json:"version" yaml:"version"`
	Profile    string     `json:"profile" yaml:"profile"`
	Count      int        `json:"count" yaml:"count"`
	CPU        int        `json:"cpu" yaml:"cpu"`
	MemoryMiB  int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int        `json:"diskGiB" yaml:"diskGiB"`
	Disks      []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs       []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	IPBase     string     `json:"ipBase" yaml:"ipBase"`
	MACPrefix  string     `json:"macPrefix" yaml:"macPrefix"`
	APIVIP     string     `json:"apiVIP" yaml:"apiVIP"`
	IngressVIP string     `json:"ingressVIP" yaml:"ingressVIP"`
	// Per-cluster lifecycle gaps / status (each DEPLOYMENT-CLUSTER is its own object).
	Phase           string `json:"phase,omitempty" yaml:"phase,omitempty"` // planned | created | installing | ready | destroy
	ClusterImageSet string `json:"clusterImageSet,omitempty" yaml:"clusterImageSet,omitempty"`
	DiscoveryISO    string `json:"discoveryISO,omitempty" yaml:"discoveryISO,omitempty"`
	Notes           string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// GapParams captures shared hub MVP-gap inputs (cluster-specific gaps live on ClusterNode).
type GapParams struct {
	PullSecretFile   string `json:"pullSecretFile" yaml:"pullSecretFile"`
	SSHPublicKeyFile string `json:"sshPublicKeyFile" yaml:"sshPublicKeyFile"`
	HubKubeconfig    string `json:"hubKubeconfig" yaml:"hubKubeconfig"`
	ManualApprove    bool   `json:"manualApprove" yaml:"manualApprove"`
	// Deprecated shared fields — kept for older mockups; prefer ClusterNode fields.
	ClusterImageSet string `json:"clusterImageSet,omitempty" yaml:"clusterImageSet,omitempty"`
	DiscoveryISO    string `json:"discoveryISO,omitempty" yaml:"discoveryISO,omitempty"`
}

type Status struct {
	Phase               Phase  `json:"phase" yaml:"phase"`
	Message             string `json:"message,omitempty" yaml:"message,omitempty"`
	CheapcloudProductID  string `json:"cheapcloudProductId,omitempty" yaml:"cheapcloudProductId,omitempty"`
	CheapcloudTrackedAt  string `json:"cheapcloudTrackedAt,omitempty" yaml:"cheapcloudTrackedAt,omitempty"`
}

// Layout stores SVG canvas positions (interview-me mind-map style).
type Layout struct {
	Nodes map[string]NodePos `json:"nodes" yaml:"nodes"`
}

type NodePos struct {
	X float64 `json:"x" yaml:"x"`
	Y float64 `json:"y" yaml:"y"`
}

// Store persists mockups under dataDir/mockups/<id>/mockup.yaml.
type Store struct {
	root string
}

func NewStore(dataDir string) (*Store, error) {
	root := filepath.Join(dataDir, "mockups")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

type CreateReq struct {
	Name       string `json:"name"`
	BaseDomain string `json:"baseDomain"`
	Provider   string `json:"provider"`
	Notes      string `json:"notes"`
	Genre      string `json:"genre"`
	Style      string `json:"style"`
	// SeedDevLab writes throwaway SSH/pull-secret/ISO stubs under mockups/<id>/dev-lab/
	// for hands-free "Use defaults" Validate → Deploy click-through (LAB/TEST/DEV ONLY).
	SeedDevLab bool `json:"seedDevLab"`
}

func (s *Store) Create(req CreateReq) (*MockUp, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name required")
	}
	if req.BaseDomain == "" {
		req.BaseDomain = "lab.example.net"
	}
	if req.Provider == "" {
		req.Provider = "libvirt"
	}
	genre, style, _, err := ResolveCreateStyle(req.Genre, req.Style)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	m := seedMockUp(style, id, req.Name, req.BaseDomain, req.Provider, req.Notes, now)
	m.Spec.Genre = genre
	m.Spec.Style = style
	if err := s.save(m); err != nil {
		return nil, err
	}
	if req.SeedDevLab {
		m, err = s.SeedDevLabGaps(id)
		if err != nil {
			_ = s.Delete(id)
			return nil, fmt.Errorf("seed DEV lab gaps: %w", err)
		}
	}
	return m, nil
}

// seedMockUp builds the default canvas for a creatable style.
func seedMockUp(style, id, name, domain, provider, notes, now string) *MockUp {
	switch style {
	case StyleSingleSNOOCP:
		return defaultSingleSNOMockUp(id, name, domain, provider, notes, now)
	case StyleCloudCostModel:
		return defaultCloudCostModel(id, name, domain, provider, notes, now)
	case StyleAROAzureLab:
		return defaultAROAzureLab(id, name, domain, provider, notes, now)
	default:
		return defaultMockUp(id, name, domain, provider, notes, now)
	}
}

func defaultAROAzureLab(id, name, domain, provider, notes, now string) *MockUp {
	if provider == "" || provider == "libvirt" {
		provider = "azure"
	}
	if notes == "" {
		notes = "Cheapest viable ARO: 3×D8s_v3 masters + 3×D4s_v3 workers + 2 Spot workers. Deploy via ARM/az aro; Spot via MachineSet."
	}
	return &MockUp{
		APIVersion: "mock-me.dasmlab.org/v1alpha1",
		Kind:       "MockUp",
		Metadata: Metadata{
			ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Notes: notes,
		},
		Spec: Spec{
			Genre:      GenreInfrastructure,
			Style:      StyleAROAzureLab,
			BaseDomain: domain,
			Provider:   provider,
			CanvasMode: "freeform",
			Canvas: &CanvasSpec{
				ShowRelations: true,
				OmitHost:      true,
				OmitGateway:   true,
				OmitHub:       true,
				OmitACM:       true,
				Orphans: []CanvasNode{
					{ID: "aro-vnet", Kind: "cloud-vnet", Label: "aro-vnet", Notes: "cidr=10.100.0.0/15", X: 60, Y: 40},
					{ID: "subnet-master", Kind: "cloud-subnet", Label: "master", Notes: "cidr=10.100.76.0/24 role=master", X: 60, Y: 140},
					{ID: "subnet-worker", Kind: "cloud-subnet", Label: "worker", Notes: "cidr=10.100.70.0/23 role=worker", X: 60, Y: 220},
					{ID: "aro-cluster", Kind: "cloud-aro-cluster", Label: "ARO cluster", Notes: "api=Public ingress=Public version=latest region=eastus", X: 320, Y: 40},
					{ID: "master-pool", Kind: "cloud-aro-master", Label: "3× masters", Notes: "sku=Standard_D8s_v3 count=3 spot=false", X: 320, Y: 140},
					{ID: "worker-pool", Kind: "cloud-aro-worker", Label: "3× workers", Notes: "sku=Standard_D4s_v3 count=3 spot=false disk=128", X: 320, Y: 220},
					{ID: "spot-pool", Kind: "cloud-aro-spot-worker", Label: "2× Spot workers", Notes: "sku=Standard_D4s_v3 count=2 spot=true taint=spot=true:NoExecute", X: 560, Y: 220},
				},
			},
			Network: NetworkSpec{
				MachineCIDR: "10.100.0.0/15",
				Gateway:     "10.100.0.1",
			},
			InfraHost: InfraHostNode{ID: "infra-host", Label: "unused", Hostname: "n/a"},
			Gateway:   GatewayNode{ID: "gateway", Label: "unused"},
			Hub:       HubNode{ID: "hub", Label: "unused"},
			ACM:       ACMNode{Enabled: false},
			Gaps: GapParams{
				PullSecretFile: "pull-secret.txt",
			},
		},
		Status: Status{Phase: PhaseCreated, CheapcloudProductID: "mock-me-" + id},
		Layout: Layout{Nodes: map[string]NodePos{
			"aro-vnet":       {X: 60, Y: 40},
			"subnet-master":  {X: 60, Y: 140},
			"subnet-worker":  {X: 60, Y: 220},
			"aro-cluster":    {X: 320, Y: 40},
			"master-pool":    {X: 320, Y: 140},
			"worker-pool":    {X: 320, Y: 220},
			"spot-pool":      {X: 560, Y: 220},
		}},
	}
}

func defaultCloudCostModel(id, name, domain, provider, notes, now string) *MockUp {
	if provider == "" || provider == "libvirt" {
		provider = "multi-cloud"
	}
	return &MockUp{
		APIVersion: "mock-me.dasmlab.org/v1alpha1",
		Kind:       "MockUp",
		Metadata: Metadata{
			ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Notes: notes,
		},
		Spec: Spec{
			Genre:      GenreInfrastructure,
			Style:      StyleCloudCostModel,
			BaseDomain: domain,
			Provider:   provider,
			CanvasMode: "freeform",
			Canvas: &CanvasSpec{
				ShowRelations: false,
				OmitHost:      true,
				OmitGateway:   true,
				OmitHub:       true,
				OmitACM:       true,
				Orphans: []CanvasNode{
					{ID: "vnet-1", Kind: "cloud-vnet", Label: "cc-vnet", Notes: "cidr=10.42.0.0/16", X: 80, Y: 80},
					{ID: "sno-1", Kind: "cloud-ocp-sno-slim", Label: "OCP SNO slim (Spot)", Notes: "sku=Standard_D8s_v3", X: 320, Y: 140},
				},
			},
			Network: NetworkSpec{
				MachineCIDR: "10.42.0.0/16",
				Gateway:     "10.42.0.1",
			},
			InfraHost: InfraHostNode{ID: "infra-host", Label: "unused", Hostname: "n/a"},
			Gateway:   GatewayNode{ID: "gateway", Label: "unused"},
			Hub:       HubNode{ID: "hub", Label: "unused"},
			ACM:       ACMNode{Enabled: false},
			Clusters:  nil,
		},
		Status: Status{Phase: PhaseCreated, CheapcloudProductID: "mock-me-" + id},
		Layout: Layout{Nodes: map[string]NodePos{
			"vnet-1": {X: 80, Y: 80},
			"sno-1":  {X: 320, Y: 140},
		}},
	}
}

func defaultSingleSNOMockUp(id, name, domain, provider, notes, now string) *MockUp {
	infraID := "infra-host"
	gwID := "gateway"
	hubID := "hub"
	gw := defaultGateway()
	return &MockUp{
		APIVersion: "mock-me.dasmlab.org/v1alpha1",
		Kind:       "MockUp",
		Metadata: Metadata{
			ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Notes: notes,
		},
		Spec: Spec{
			Genre:      GenreClusterManagement,
			Style:      StyleSingleSNOOCP,
			BaseDomain: domain,
			Provider:   provider,
			Network: NetworkSpec{
				MachineCIDR: "10.77.30.0/24",
				Gateway:     "10.77.30.1",
				APIVIP:      "10.77.30.20",
				IngressVIP:  "10.77.30.21",
				DHCPStart:   "10.77.30.100",
				DHCPEnd:     "10.77.30.150",
				DNS:         "10.77.30.1",
			},
			InfraHost: defaultInfraHost(name),
			Gateway:   gw,
			Hub: HubNode{
				ID: hubID, Label: "OCP-MGMT", Mode: "local-agent",
				Version: "4.18", Profile: "hub-supported",
				Hostname: "sno", IP: "10.77.30.20", MAC: "52:54:00:13:00:20",
				CPU: 8, MemoryMiB: 24576, DiskGiB: 200, InstallACM: false,
				Disks: []DiskSpec{{Name: "vda", SizeGiB: 200, Bus: "virtio", Role: "system"}},
				NICs: []NICSpec{{
					Name: "eth0", Model: "virtio", Mode: "libvirt-network",
					Network: "ocp-lab", Role: "guest",
				}},
			},
			ACM: ACMNode{
				ID: "acm", Label: "ACM", Enabled: false,
				MCEChannel: "stable-2.7", ACMChannel: "release-2.12",
			},
			Clusters: []ClusterNode{},
			Canvas: &CanvasSpec{
				OmitACM: true, // style stops before ACM
			},
			Gaps: GapParams{
				ManualApprove:    true,
				PullSecretFile:   "$PULL_SECRET_FILE",
				SSHPublicKeyFile: "$SSH_PUBLIC_KEY_FILE",
				HubKubeconfig:    fmt.Sprintf("./data/hub-%s/auth/kubeconfig", name),
			},
		},
		Status: Status{
			Phase:   PhaseCreated,
			Message: "Single SNO MockUp — MACHINE-HOST + VYOS-GW + OCP-MGMT (no ACM / deployments)",
		},
		Layout: Layout{Nodes: map[string]NodePos{
			infraID: {X: 120, Y: 500},
			gwID:    {X: 200, Y: 340},
			hubID:   {X: 420, Y: 280},
		}},
	}
}

func defaultMockUp(id, name, domain, provider, notes, now string) *MockUp {
	infraID := "infra-host"
	gwID := "gateway"
	hubID := "hub"
	acmID := "acm"
	// Lab LAN behind VyOS (obscure private — not home LAN, not VMnet12).
	c0 := newClusterNode(0, "4.18", "10.77.30.10", "10.77.30.11")
	c1 := newClusterNode(1, "4.18", "10.77.30.12", "10.77.30.13")
	gw := defaultGateway()
	return &MockUp{
		APIVersion: "mock-me.dasmlab.org/v1alpha1",
		Kind:       "MockUp",
		Metadata: Metadata{
			ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Notes: notes,
		},
		Spec: Spec{
			Genre:      GenreClusterManagement,
			Style:      StyleACMMultiCluster,
			BaseDomain: domain,
			Provider:   provider,
			Network: NetworkSpec{
				MachineCIDR: "10.77.30.0/24",
				Gateway:     "10.77.30.1",
				APIVIP:      "10.77.30.10",
				IngressVIP:  "10.77.30.11",
				DHCPStart:   "10.77.30.100",
				DHCPEnd:     "10.77.30.150",
				DNS:         "10.77.30.1",
			},
			InfraHost: defaultInfraHost(name),
			Gateway:   gw,
			Hub: HubNode{
				ID: hubID, Label: "OCP-MGMT", Mode: "local-agent",
				Version: "4.18", Profile: "hub-supported",
				Hostname: "hub-sno", IP: "10.77.30.20", MAC: "52:54:00:13:00:20",
				CPU: 8, MemoryMiB: 24576, DiskGiB: 200, InstallACM: true,
				Disks: []DiskSpec{{Name: "vda", SizeGiB: 200, Bus: "virtio", Role: "system"}},
				NICs: []NICSpec{{
					Name: "eth0", Model: "virtio", Mode: "libvirt-network",
					Network: "ocp-lab", Role: "guest",
				}},
			},
			ACM: ACMNode{
				ID: acmID, Label: "ACM", Enabled: true,
				MCEChannel: "stable-2.7", ACMChannel: "release-2.12",
			},
			Clusters: []ClusterNode{c0, c1},
			Gaps: GapParams{
				ManualApprove:    true,
				PullSecretFile:   "$PULL_SECRET_FILE",
				SSHPublicKeyFile: "$SSH_PUBLIC_KEY_FILE",
				HubKubeconfig:    fmt.Sprintf("./data/hub-%s/auth/kubeconfig", name),
			},
		},
		Status: Status{Phase: PhaseCreated, Message: "MockUp created — MACHINE-HOST + VYOS-GW + OCP-MGMT + ACM + OCP-DEPLOY clusters"},
		Layout: Layout{Nodes: map[string]NodePos{
			infraID: {X: 120, Y: 500},
			gwID:    {X: 140, Y: 340},
			hubID:   {X: 340, Y: 300},
			acmID:   {X: 340, Y: 150},
			c0.ID:   {X: 580, Y: 180},
			c1.ID:   {X: 580, Y: 320},
		}},
	}
}

func defaultInfraHost(rackName string) InfraHostNode {
	host := "rhel10-vhost-mock-me"
	if rackName != "" {
		host = "prov-" + rackName
	}
	// Nested RHEL 10 MACHINE HOST: OS disk + libvirt pool disk; bridged uplink + optional host-only.
	disks := []DiskSpec{
		{Name: "nvme0", SizeGiB: 250, Bus: "nvme", Role: "system"}, // OS/logs only
		{Name: "nvme1", SizeGiB: 400, Bus: "nvme", Role: "pool"},   // guest VM storage
	}
	return InfraHostNode{
		ID: "infra-host", Label: "MACHINE-HOST",
		Hostname: host, Kind: "nested-vm", Hypervisor: "vmware",
		OS: "rhel-10", Arch: "x86_64",
		CPU: 24, MemoryMiB: 40960,
		Disks: disks, DiskGiB: sumDiskGiB(disks),
		NICs: []NICSpec{
			{Name: "ens192", Model: "e1000e", Mode: "bridged", Network: "bridged-auto", Role: "uplink"},
			// VMnet12 host-only — reserved for future "move up a class"; not the lab LAN.
			{Name: "ens224", Model: "e1000e", Mode: "host-only", Network: "VMnet12", Role: "host-only"},
		},
		LibvirtURI: "qemu:///system", NetworkName: "ocp-lab", StoragePool: "mock-me",
		Podman:       true,
		SSHHost:      "192.168.1.142",
		SSHUser:      "dasm",
		ACMReference: "BareMetalHost analogue — libvirtd host; guests install via InfraEnv (agentBareMetal)",
		Notes:        "RHEL MACHINE HOST (BM or nested). Bridged NIC for SSH/mgmt; host-only VMnet12 optional. Libvirt LAN is behind VYOS-GW, not on VMnet12. Large disk = guest pool. Link Inventory entry for probe/orchestrate.",
	}
}

func defaultGateway() GatewayNode {
	disks := []DiskSpec{{Name: "vda", SizeGiB: 10, Bus: "virtio", Role: "system"}}
	return GatewayNode{
		ID: "gateway", Label: "VYOS-GW", Hostname: "vyos-lab-gw",
		Image: "vyos", CPU: 2, MemoryMiB: 2048, DiskGiB: 10, Disks: disks,
		NICs: []NICSpec{
			{Name: "eth0", Model: "virtio", Mode: "bridged", Network: "bridged-auto", Role: "wan"},
			{Name: "eth1", Model: "virtio", Mode: "libvirt-network", Network: "ocp-lab", Role: "lan"},
		},
		WANBridge: "bridged-auto", LANNetwork: "ocp-lab",
		LANCIDR: "10.77.30.0/24", LANIP: "10.77.30.1",
		NAT: true, Firewall: true, Phase: "planned",
		Notes: "Boot VyOS ISO on MACHINE-HOST. eth0=WAN (host bridge), eth1=LAN (libvirt ocp-lab). Hub + deployment guests live on LAN; NAT/FW out of band for now.",
	}
}

func newClusterNode(index int, version, apiVIP, ingressVIP string) ClusterNode {
	n := index + 1
	id := fmt.Sprintf("cluster-%d", index)
	disks := []DiskSpec{{Name: "vda", SizeGiB: 120, Bus: "virtio", Role: "system"}}
	return ClusterNode{
		ID: id, Label: fmt.Sprintf("DEPLOYMENT-CLUSTER-%d", n),
		Name:    fmt.Sprintf("dev%02d", n),
		Version: version, Profile: "supported", Count: 3,
		CPU: 4, MemoryMiB: 16384, DiskGiB: 120, Disks: disks,
		NICs: []NICSpec{
			{Name: "eth0", Model: "virtio", Mode: "libvirt-network", Network: "ocp-lab", Role: "guest"},
		},
		IPBase:          fmt.Sprintf("10.77.30.%d", 21+(index*10)),
		MACPrefix:       fmt.Sprintf("52:54:00:%02x:00", 0x13+index),
		APIVIP:          apiVIP,
		IngressVIP:      ingressVIP,
		Phase:           "planned",
		ClusterImageSet: ImageSetName(version),
	}
}

func sumDiskGiB(disks []DiskSpec) int {
	total := 0
	for _, d := range disks {
		total += d.SizeGiB
	}
	return total
}

func ensureGuestDisksNICs(diskGiB int, network string) ([]DiskSpec, []NICSpec, int) {
	if diskGiB <= 0 {
		diskGiB = 120
	}
	if network == "" {
		network = "ocp-lab"
	}
	disks := []DiskSpec{{Name: "vda", SizeGiB: diskGiB, Bus: "virtio", Role: "system"}}
	nics := []NICSpec{{Name: "eth0", Model: "virtio", Mode: "libvirt-network", Network: network, Role: "guest"}}
	return disks, nics, diskGiB
}

// ImageSetName derives a conventional ClusterImageSet name from an OCP version.
func ImageSetName(version string) string {
	compact := strings.ReplaceAll(version, ".", "")
	if compact == "" {
		compact = "418"
	}
	return fmt.Sprintf("img%s-x86-64-appsub", compact)
}

func (s *Store) List() ([]*MockUp, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []*MockUp
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Store) Get(id string) (*MockUp, error) {
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var m MockUp
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	normalize(&m)
	return &m, nil
}

func (s *Store) Save(m *MockUp) error {
	normalize(m)
	m.Metadata.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.save(m)
}

func (s *Store) Delete(id string) error {
	return os.RemoveAll(filepath.Join(s.root, id))
}

func (s *Store) path(id string) string {
	return filepath.Join(s.root, id, "mockup.yaml")
}

func (s *Store) save(m *MockUp) error {
	dir := filepath.Join(s.root, m.Metadata.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(m.Metadata.ID), b, 0o644)
}

// Dir returns the mockup directory.
func (s *Store) Dir(id string) string {
	return filepath.Join(s.root, id)
}

// DataRoot is the mock-me data directory (parent of mockups/).
func (s *Store) DataRoot() string {
	return filepath.Dir(s.root)
}

// normalize fills MVP-gap defaults and migrates legacy shared cluster gaps.
func normalize(m *MockUp) {
	if m.Spec.Genre == "" {
		m.Spec.Genre = GenreClusterManagement
	}
	if m.Spec.Style == "" || m.Spec.Style == "mini-acm-multi-cluster" {
		m.Spec.Style = StyleACMMultiCluster
	}
	// Migrate legacy API groups from prior product names.
	switch m.APIVersion {
	case "", "mini-acm.dasmlab.org/v1alpha1", "mini-mock.dasmlab.org/v1alpha1":
		m.APIVersion = "mock-me.dasmlab.org/v1alpha1"
	}
	if m.Spec.CanvasMode == "" {
		m.Spec.CanvasMode = "guided"
	}
	if m.Spec.Gaps.PullSecretFile == "" {
		m.Spec.Gaps.PullSecretFile = "$PULL_SECRET_FILE"
	}
	if m.Spec.Gaps.SSHPublicKeyFile == "" {
		m.Spec.Gaps.SSHPublicKeyFile = "$SSH_PUBLIC_KEY_FILE"
	}
	if m.Spec.Gaps.HubKubeconfig == "" && m.Metadata.Name != "" {
		m.Spec.Gaps.HubKubeconfig = fmt.Sprintf("./data/hub-%s/auth/kubeconfig", m.Metadata.Name)
	}
	if m.Spec.InfraHost.ID == "" {
		m.Spec.InfraHost = defaultInfraHost(m.Metadata.Name)
		if m.Layout.Nodes == nil {
			m.Layout.Nodes = map[string]NodePos{}
		}
		if _, ok := m.Layout.Nodes[m.Spec.InfraHost.ID]; !ok {
			m.Layout.Nodes[m.Spec.InfraHost.ID] = NodePos{X: 120, Y: 500}
		}
	}
	if m.Spec.Gateway.ID == "" {
		m.Spec.Gateway = defaultGateway()
		if m.Layout.Nodes == nil {
			m.Layout.Nodes = map[string]NodePos{}
		}
		if _, ok := m.Layout.Nodes[m.Spec.Gateway.ID]; !ok {
			m.Layout.Nodes[m.Spec.Gateway.ID] = NodePos{X: 140, Y: 340}
		}
	}
	ih := &m.Spec.InfraHost
	if ih.Label == "" || ih.Label == "INFRA-HOST" {
		ih.Label = "MACHINE-HOST"
	}
	if ih.Kind == "" {
		ih.Kind = "nested-vm"
	}
	if ih.Kind == "nested-vm" && ih.Hypervisor == "" {
		ih.Hypervisor = "vmware"
	}
	if ih.OS == "" {
		ih.OS = "rhel-10"
	}
	if ih.Arch == "" {
		ih.Arch = "x86_64"
	}
	if ih.LibvirtURI == "" {
		ih.LibvirtURI = "qemu:///system"
	}
	if ih.NetworkName == "" {
		ih.NetworkName = "ocp-lab"
	}
	if ih.StoragePool == "" || ih.StoragePool == "default" {
		ih.StoragePool = "mock-me"
	}
	if ih.SSHHost == "" {
		ih.SSHHost = "192.168.1.142"
	}
	if ih.SSHUser == "" {
		ih.SSHUser = "dasm"
	}
	if ih.ACMReference == "" {
		ih.ACMReference = "BareMetalHost analogue — libvirtd host; guests install via InfraEnv (agentBareMetal)"
	}
	if len(ih.Disks) == 0 {
		if ih.DiskGiB > 0 {
			ih.Disks = []DiskSpec{{Name: "disk0", SizeGiB: ih.DiskGiB, Bus: "virtio", Role: "system"}}
		} else {
			ih.Disks = []DiskSpec{
				{Name: "nvme0", SizeGiB: 250, Bus: "nvme", Role: "system"},
				{Name: "nvme1", SizeGiB: 400, Bus: "nvme", Role: "pool"},
			}
		}
	}
	ih.DiskGiB = sumDiskGiB(ih.Disks)
	if len(ih.NICs) == 0 {
		ih.NICs = []NICSpec{
			{Name: "ens192", Model: "e1000e", Mode: "bridged", Network: "bridged-auto", Role: "uplink"},
			{Name: "ens224", Model: "e1000e", Mode: "host-only", Network: "VMnet12", Role: "host-only"},
		}
	}

	gw := &m.Spec.Gateway
	if gw.Label == "" {
		gw.Label = "VYOS-GW"
	}
	if gw.Hostname == "" {
		gw.Hostname = "vyos-lab-gw"
	}
	if gw.Image == "" {
		gw.Image = "vyos"
	}
	if gw.LANNetwork == "" {
		gw.LANNetwork = or(ih.NetworkName, "ocp-lab")
	}
	if gw.LANCIDR == "" {
		gw.LANCIDR = or(m.Spec.Network.MachineCIDR, "10.77.30.0/24")
	}
	if gw.LANIP == "" {
		gw.LANIP = or(m.Spec.Network.Gateway, "10.77.30.1")
	}
	if gw.WANBridge == "" {
		gw.WANBridge = "bridged-auto"
	}
	if gw.Phase == "" {
		gw.Phase = "planned"
	}
	if gw.CPU == 0 {
		gw.CPU = 2
	}
	if gw.MemoryMiB == 0 {
		gw.MemoryMiB = 2048
	}
	if len(gw.Disks) == 0 {
		if gw.DiskGiB == 0 {
			gw.DiskGiB = 10
		}
		gw.Disks = []DiskSpec{{Name: "vda", SizeGiB: gw.DiskGiB, Bus: "virtio", Role: "system"}}
	} else {
		gw.DiskGiB = sumDiskGiB(gw.Disks)
	}
	if len(gw.NICs) == 0 {
		gw.NICs = []NICSpec{
			{Name: "eth0", Model: "virtio", Mode: "bridged", Network: gw.WANBridge, Role: "wan"},
			{Name: "eth1", Model: "virtio", Mode: "libvirt-network", Network: gw.LANNetwork, Role: "lan"},
		}
	}
	// Keep lab NetworkSpec aligned with gateway LAN when still on legacy defaults.
	if m.Spec.Network.MachineCIDR == "" || m.Spec.Network.MachineCIDR == "192.168.130.0/24" {
		m.Spec.Network.MachineCIDR = gw.LANCIDR
		m.Spec.Network.Gateway = gw.LANIP
		m.Spec.Network.DNS = gw.LANIP
	}

	h := &m.Spec.Hub
	if len(h.Disks) == 0 || len(h.NICs) == 0 {
		disks, nics, total := ensureGuestDisksNICs(h.DiskGiB, m.Spec.InfraHost.NetworkName)
		if len(h.Disks) == 0 {
			h.Disks = disks
			h.DiskGiB = total
		}
		if len(h.NICs) == 0 {
			h.NICs = nics
		}
	} else {
		h.DiskGiB = sumDiskGiB(h.Disks)
	}

	for i := range m.Spec.Clusters {
		c := &m.Spec.Clusters[i]
		if c.Phase == "" {
			c.Phase = "planned"
		}
		if c.ClusterImageSet == "" {
			if m.Spec.Gaps.ClusterImageSet != "" {
				c.ClusterImageSet = m.Spec.Gaps.ClusterImageSet
			} else {
				c.ClusterImageSet = ImageSetName(c.Version)
			}
		}
		if c.DiscoveryISO == "" && m.Spec.Gaps.DiscoveryISO != "" {
			c.DiscoveryISO = m.Spec.Gaps.DiscoveryISO
		}
		if c.Count == 0 {
			c.Count = 3
		}
		if len(c.Disks) == 0 || len(c.NICs) == 0 {
			disks, nics, total := ensureGuestDisksNICs(c.DiskGiB, m.Spec.InfraHost.NetworkName)
			if len(c.Disks) == 0 {
				c.Disks = disks
				c.DiskGiB = total
			}
			if len(c.NICs) == 0 {
				c.NICs = nics
			}
		} else {
			c.DiskGiB = sumDiskGiB(c.Disks)
		}
	}
}

// AddCluster appends a DEPLOYMENT-CLUSTER lifecycle node (VMs + ACM CRs).
func (m *MockUp) AddCluster() ClusterNode {
	index := nextClusterIndex(m.Spec.Clusters)
	apiOctet := 10 + index*2
	ingOctet := apiOctet + 1
	c := newClusterNode(
		index,
		m.Spec.Hub.Version,
		fmt.Sprintf("10.77.30.%d", apiOctet),
		fmt.Sprintf("10.77.30.%d", ingOctet),
	)
	m.Spec.Clusters = append(m.Spec.Clusters, c)
	if m.Layout.Nodes == nil {
		m.Layout.Nodes = map[string]NodePos{}
	}
	m.Layout.Nodes[c.ID] = NodePos{X: 480, Y: float64(160 + index*100)}
	return c
}

// RemoveCluster deletes a DEPLOYMENT-CLUSTER by id (underlying VM lifecycle object).
// Guided mode keeps ≥1 cluster; free-form may remove all (teaching incomplete racks).
func (m *MockUp) RemoveCluster(clusterID string) error {
	if !m.IsFreeForm() && len(m.Spec.Clusters) <= 1 {
		return fmt.Errorf("keep at least one deployment cluster")
	}
	idx := -1
	for i, c := range m.Spec.Clusters {
		if c.ID == clusterID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("cluster %q not found", clusterID)
	}
	m.Spec.Clusters = append(m.Spec.Clusters[:idx], m.Spec.Clusters[idx+1:]...)
	if m.Layout.Nodes != nil {
		delete(m.Layout.Nodes, clusterID)
	}
	return nil
}

// IsFreeForm reports creative / teaching canvas mode.
func (m *MockUp) IsFreeForm() bool {
	return strings.EqualFold(m.Spec.CanvasMode, "freeform")
}

func (m *MockUp) canvas() *CanvasSpec {
	if m.Spec.Canvas == nil {
		m.Spec.Canvas = &CanvasSpec{}
	}
	return m.Spec.Canvas
}

func (m *MockUp) EffectiveHost() bool {
	return m.Spec.InfraHost.ID != "" && (m.Spec.Canvas == nil || !m.Spec.Canvas.OmitHost)
}

func (m *MockUp) EffectiveGateway() bool {
	return m.Spec.Gateway.ID != "" && (m.Spec.Canvas == nil || !m.Spec.Canvas.OmitGateway)
}

func (m *MockUp) EffectiveHub() bool {
	return m.Spec.Hub.ID != "" && (m.Spec.Canvas == nil || !m.Spec.Canvas.OmitHub)
}

func (m *MockUp) EffectiveACM() bool {
	return m.Spec.ACM.ID != "" && m.Spec.ACM.Enabled && (m.Spec.Canvas == nil || !m.Spec.Canvas.OmitACM)
}

func nextClusterIndex(clusters []ClusterNode) int {
	used := map[int]bool{}
	for _, c := range clusters {
		var n int
		if _, err := fmt.Sscanf(c.ID, "cluster-%d", &n); err == nil {
			used[n] = true
		}
	}
	for i := 0; i < 64; i++ {
		if !used[i] {
			return i
		}
	}
	return len(clusters)
}

// ApplyProfileSizes updates CPU/RAM/disk from known profile names.
func ApplyHubProfile(h *HubNode) {
	switch h.Profile {
	case "hub-lab", "lab", "lab-tight":
		h.CPU, h.MemoryMiB, h.DiskGiB = 8, 16384, 160
	default:
		h.CPU, h.MemoryMiB, h.DiskGiB = 8, 24576, 200
	}
}

func ApplyClusterProfile(c *ClusterNode) {
	switch c.Profile {
	case "lab-small", "lab", "unsupported":
		c.CPU, c.MemoryMiB, c.DiskGiB = 4, 12288, 120
	default:
		c.CPU, c.MemoryMiB, c.DiskGiB = 4, 16384, 120
	}
}
