package mockup

import "fmt"

// Genre / Style identify which product offering a MockUp belongs to.
// Resource kind stays "MockUp"; genre is the product family, style is the template.
const (
	GenreClusterManagement      = "cluster-management"
	GenreApplicationDevelopment = "application-development"
	GenreInfrastructure         = "infrastructure"
	GenreContentManagement      = "content-management"
	// GenreContentDelivery is a legacy alias id kept for existing mockups / docs.
	GenreContentDelivery = GenreContentManagement

	StyleACMMultiCluster      = "acm-multi-cluster"
	StyleSingleSNOOCP         = "single-sno-ocp"
	StyleWindowsUI            = "windows-ui"
	StyleWebFullStack         = "web-full-stack"
	StyleInfraNodeNetwork     = "infra-node-network-payload"
	StyleCloudCostModel       = "cloud-cost-model" // Design-bench port: multi-cloud price+track palette
	StyleAROAzureLab          = "aro-azure-lab"    // cheapest viable ARO (+ optional Spot MachineSet)
	StyleSelfServePersonalCDN = "self-serve-cloud-personal-cdn"
	StyleSurfingCdnR2         = "surfing-cdn-r2" // golden implementation example of Self-Serve Personal CDN
)

// RelationRule constrains how object types may connect (validate / palette later).
type RelationRule struct {
	From        string `json:"from"`
	Rel         string `json:"rel"`
	To          string `json:"to"`
	Cardinality string `json:"cardinality,omitempty"` // e.g. "1..1", "1..*", "0..*"
	Notes       string `json:"notes,omitempty"`
}

// StyleDef is a creatable (or stub) template within a genre.
type StyleDef struct {
	ID          string         `json:"id"`
	Genre       string         `json:"genre"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Available   bool           `json:"available"` // false = catalog stub, create rejected
	ObjectTypes []string       `json:"objectTypes"`
	Views       []string       `json:"views,omitempty"`
	Relations   []RelationRule `json:"relations,omitempty"`
	DefaultSeed string         `json:"defaultSeed,omitempty"` // human hint
}

// GenreDef groups styles.
type GenreDef struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Styles      []string `json:"styles"` // style ids
}

// CatalogResponse is GET /api/v1/catalog.
type CatalogResponse struct {
	Genres []GenreDef `json:"genres"`
	Styles []StyleDef `json:"styles"`
}

// Catalog returns the product genre/style registry (code-defined for now;
// later: data/genres/*.yaml + "Add a Genre" UI).
func Catalog() CatalogResponse {
	styles := []StyleDef{
		{
			ID: StyleSingleSNOOCP, Genre: GenreClusterManagement,
			Label:       "Single SNO OCP",
			Description: "Bring up one SNO (OCP-MGMT) via the adapter (libvirt today). Same MACHINE-HOST → vHost path as the mgmt half of ACM Multi-Cluster — stop before ACM / spokes.",
			Available:   true,
			ObjectTypes: []string{
				"MachineHost", "Adapter", "VHost", "Gateway", "OCP-MGMT", "Appliance",
			},
			Views:       []string{"all", "infra", "network", "cluster"},
			DefaultSeed: "lab rack with MACHINE-HOST + VyOS + one SNO OCP-MGMT (no ACM, no deployments)",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Gateway", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "VyOS on vHost-GW"},
				{From: "OCP-MGMT", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "SNO guest — same Hub object used by ACM Multi-Cluster"},
			},
		},
		{
			ID: StyleACMMultiCluster, Genre: GenreClusterManagement,
			Label:       "ACM Multi-Cluster",
			Description: "Composable: Single SNO (OCP-MGMT) + ACM payload + N× OCP-DEPLOY. Mgmt hosts ACM; managed deployments on the lab rack.",
			Available:   true,
			ObjectTypes: []string{
				"MachineHost", "Adapter", "VHost", "Gateway", "OCP-MGMT", "ACM", "OCP-DEPLOY", "Appliance",
			},
			Views:       []string{"all", "infra", "network", "cluster", "app"},
			DefaultSeed: "lab rack with OCP-MGMT + ACM + 2 OCP-DEPLOY clusters",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Gateway", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "VyOS on vHost-GW"},
				{From: "OCP-MGMT", Rel: "runsOn", To: "VHost", Cardinality: "1..1", Notes: "MGMT SNO guest"},
				{From: "OCP-DEPLOY", Rel: "runsOn", To: "VHost", Cardinality: "1..*", Notes: "cp/worker guests"},
				{From: "ACM", Rel: "runsOn", To: "OCP-MGMT", Cardinality: "1..1"},
				{From: "OCP-DEPLOY", Rel: "managedBy", To: "ACM", Cardinality: "1..*"},
			},
		},
		{
			ID: StyleWindowsUI, Genre: GenreApplicationDevelopment,
			Label:       "Windows UI MockUp",
			Description: "Client app SDLC canvas: OS → runtime (.NET/WPF, …) → UI surfaces → data/devices/services. Inspired by apps like running-translate.",
			Available:   false,
			ObjectTypes: []string{
				"RunningOS", "ClientRuntime", "Window", "Form", "Control", "DataInput", "Device", "DataOutput", "ServiceCall",
			},
			Views:       []string{"all", "runtime", "ui", "dataflow"},
			DefaultSeed: "stub — empty Windows UI canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "ClientRuntime", Rel: "runsOn", To: "RunningOS", Cardinality: "1..1"},
				{From: "Window", Rel: "hostedBy", To: "ClientRuntime", Cardinality: "1..*"},
				{From: "Form", Rel: "contains", To: "Control", Cardinality: "0..*"},
				{From: "Form", Rel: "navigatesTo", To: "Form", Cardinality: "0..*"},
				{From: "Form", Rel: "reads", To: "DataInput", Cardinality: "0..*"},
				{From: "Form", Rel: "writes", To: "DataOutput", Cardinality: "0..*"},
				{From: "Form", Rel: "calls", To: "ServiceCall", Cardinality: "0..*"},
			},
		},
		{
			ID: StyleWebFullStack, Genre: GenreApplicationDevelopment,
			Label:       "Web Full-Stack Application",
			Description: "Routes, FE/BE components, APIs, data stores — bread-and-butter app MockUps.",
			Available:   false,
			ObjectTypes: []string{
				"Route", "Frontend", "Backend", "API", "DataStore", "Auth", "ExternalService",
			},
			Views:       []string{"all", "frontend", "backend", "data"},
			DefaultSeed: "stub — empty web stack canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "Frontend", Rel: "calls", To: "API", Cardinality: "0..*"},
				{From: "Backend", Rel: "exposes", To: "API", Cardinality: "0..*"},
				{From: "Backend", Rel: "uses", To: "DataStore", Cardinality: "0..*"},
				{From: "Route", Rel: "renders", To: "Frontend", Cardinality: "1..*"},
			},
		},
		{
			ID: StyleInfraNodeNetwork, Genre: GenreInfrastructure,
			Label:       "Infra · Node · Network · Payload",
			Description: "Standalone infrastructure MockUp (hosts, nets, payloads) without ACM governance.",
			Available:   false,
			ObjectTypes: []string{"MachineHost", "Adapter", "VHost", "Network", "Appliance", "Payload"},
			Views:       []string{"all", "infra", "network", "payload"},
			DefaultSeed: "stub — infra-focused canvas (not seeded yet)",
			Relations: []RelationRule{
				{From: "Adapter", Rel: "runsOn", To: "MachineHost", Cardinality: "1..1"},
				{From: "VHost", Rel: "hostedBy", To: "Adapter", Cardinality: "1..*"},
				{From: "Payload", Rel: "runsOn", To: "VHost", Cardinality: "0..*"},
			},
		},
		{
			ID: StyleCloudCostModel, Genre: GenreInfrastructure,
			Label:       "Cloud cost model",
			Description: "Multi-cloud Design bench: Azure / AWS / GCP Spot + OCP SNO slim + R2. Cost me + Import & track into cheapcloud.",
			Available:   true,
			ObjectTypes: []string{
				"cloud-vnet", "cloud-subnet", "cloud-vm-spot", "cloud-ocp-sno-slim",
				"cloud-aws-ec2-spot", "cloud-gcp-gce-spot", "cloud-r2", "cloud-nsg",
			},
			Views:       []string{"all", "network", "compute", "storage"},
			DefaultSeed: "free-form cloud palette — multi-provider Spot / SNO / R2",
			Relations: []RelationRule{
				{From: "cloud-subnet", Rel: "in", To: "cloud-vnet", Cardinality: "0..*"},
				{From: "cloud-vm-spot", Rel: "in", To: "cloud-vnet", Cardinality: "0..*"},
				{From: "cloud-ocp-sno-slim", Rel: "in", To: "cloud-vnet", Cardinality: "0..*"},
				{From: "cloud-aws-ec2-spot", Rel: "in", To: "cloud-vnet", Cardinality: "0..*"},
				{From: "cloud-gcp-gce-spot", Rel: "in", To: "cloud-vnet", Cardinality: "0..*"},
			},
		},
		{
			ID: StyleAROAzureLab, Genre: GenreInfrastructure,
			Label:       "ARO Azure lab (cheapest)",
			Description: "Azure Red Hat OpenShift floor: RG → VNet/master+worker subnets → 3 masters (D8s_v3) + 3 on-demand workers (D4s_v3) + optional Spot MachineSet. Cost me via cheapcloud; Spot reclaim → SKU fallback.",
			Available:   true,
			ObjectTypes: []string{
				"cloud-vnet", "cloud-subnet", "cloud-aro-cluster",
				"cloud-aro-master", "cloud-aro-worker", "cloud-aro-spot-worker", "cloud-nsg",
			},
			Views:       []string{"all", "network", "compute", "cluster"},
			DefaultSeed: "ARO ARM defaults + 2 Spot workers (tainted) for interruptible play",
			Relations: []RelationRule{
				{From: "cloud-subnet", Rel: "in", To: "cloud-vnet", Cardinality: "1..*"},
				{From: "cloud-aro-master", Rel: "in", To: "cloud-subnet", Cardinality: "3..3", Notes: "never Spot"},
				{From: "cloud-aro-worker", Rel: "in", To: "cloud-subnet", Cardinality: "3..*", Notes: "≥3 non-Spot"},
				{From: "cloud-aro-spot-worker", Rel: "in", To: "cloud-subnet", Cardinality: "0..*", Notes: "MachineSet spotVMOptions"},
				{From: "cloud-aro-cluster", Rel: "uses", To: "cloud-vnet", Cardinality: "1..1"},
			},
		},
		{
			ID: StyleSelfServePersonalCDN, Genre: GenreContentManagement,
			Label:       "Self-Serve Cloud Personal CDN",
			Description: "Customer-held keys + pass-through cloud bill (~$1 service for index/site-CDN/migrate). Portal → OAuth2/SSO to CF/Azure/GCP → BYO bucket backend → Edge CDN → catalog/index (cdn-mgr). Surfing is the golden live example. cheapcloud profiles free-tier burn; mock-me templates this for cdn-mgr later.",
			Available:   false,
			ObjectTypes: []string{
				"CustomerPortal", "IdentitySSO", "KeyVault", "Realm", "Backend",
				"ObjectStore", "EdgeCDN", "SiteCDN", "WebHost", "CatalogIndex",
				"Collection", "Asset", "CostProfile", "PublishPipeline",
			},
			Views:       []string{"all", "portal", "identity", "storage", "edge", "hosting", "cost"},
			DefaultSeed: "stub — portal+SSO → BYO R2 → CF edge → index; web host on DC OCP",
			Relations: []RelationRule{
				{From: "CustomerPortal", Rel: "authenticatesVia", To: "IdentitySSO", Cardinality: "1..1", Notes: "OAuth2 / OIDC to CF, Azure, Google"},
				{From: "IdentitySSO", Rel: "grants", To: "KeyVault", Cardinality: "1..1", Notes: "user holds keys; we store refs only"},
				{From: "Realm", Rel: "ownedBy", To: "CustomerPortal", Cardinality: "1..1"},
				{From: "Backend", Rel: "boundTo", To: "Realm", Cardinality: "1..*"},
				{From: "Backend", Rel: "usesKeysFrom", To: "KeyVault", Cardinality: "1..1"},
				{From: "ObjectStore", Rel: "implements", To: "Backend", Cardinality: "1..1", Notes: "R2 / Azure Blob / GCS / Glacier adapter"},
				{From: "EdgeCDN", Rel: "pullsFrom", To: "ObjectStore", Cardinality: "1..1"},
				{From: "SiteCDN", Rel: "terminatesAt", To: "EdgeCDN", Cardinality: "1..1"},
				{From: "WebHost", Rel: "serves", To: "CustomerPortal", Cardinality: "0..1", Notes: "gallery UX — Surfing on OCP DC today"},
				{From: "WebHost", Rel: "runsOn", To: "ObjectStore", Cardinality: "0..0", Notes: "bytes are NOT on webhost — CDN/origin only"},
				{From: "CatalogIndex", Rel: "indexes", To: "Collection", Cardinality: "1..*"},
				{From: "Collection", Rel: "contains", To: "Asset", Cardinality: "0..*"},
				{From: "PublishPipeline", Rel: "publishes", To: "Asset", Cardinality: "0..*"},
				{From: "PublishPipeline", Rel: "writesTo", To: "ObjectStore", Cardinality: "1..1"},
				{From: "CostProfile", Rel: "constrains", To: "ObjectStore", Cardinality: "0..1", Notes: "cheapcloud free-tier / $ envelope"},
				{From: "CostProfile", Rel: "constrains", To: "EdgeCDN", Cardinality: "0..1"},
			},
		},
		{
			ID: StyleSurfingCdnR2, Genre: GenreContentManagement,
			Label:       "Surfing (golden Personal CDN example)",
			Description: "Live implementation slice of Self-Serve Cloud Personal CDN: dasmlab_home Surfing UX (WebHost on 2026-prod-1) → surfing-service publish → R2 dasmlab-surfing → CF pub-*.r2.dev. Keep as before/after golden client while dasmlab-cdn-mgr grows.",
			Available:   false,
			ObjectTypes: []string{
				"WebHost", "OriginApp", "ObjectStore", "EdgeCDN", "SiteCDN",
				"Collection", "Asset", "PublishPipeline", "CostProfile", "KeyVault",
			},
			Views:       []string{"all", "hosting", "storage", "edge", "cost"},
			DefaultSeed: "stub — Surfing on OCP + R2 + CF (mirrors production)",
			Relations: []RelationRule{
				{From: "OriginApp", Rel: "runsBeside", To: "WebHost", Cardinality: "1..1", Notes: "surfing-service API + dasmlab_home SPA"},
				{From: "PublishPipeline", Rel: "runsOn", To: "OriginApp", Cardinality: "1..1"},
				{From: "PublishPipeline", Rel: "writesTo", To: "ObjectStore", Cardinality: "1..1"},
				{From: "ObjectStore", Rel: "usesKeysFrom", To: "KeyVault", Cardinality: "1..1", Notes: "R2 S3 API token today; SSO later"},
				{From: "EdgeCDN", Rel: "pullsFrom", To: "ObjectStore", Cardinality: "1..1"},
				{From: "SiteCDN", Rel: "terminatesAt", To: "EdgeCDN", Cardinality: "1..1"},
				{From: "WebHost", Rel: "linksTo", To: "SiteCDN", Cardinality: "1..1", Notes: "browser loads media from CDN URLs directly"},
				{From: "Collection", Rel: "contains", To: "Asset", Cardinality: "0..*"},
				{From: "CostProfile", Rel: "constrains", To: "ObjectStore", Cardinality: "0..1"},
			},
		},
	}

	genres := []GenreDef{
		{
			ID: GenreClusterManagement, Label: "Cluster Management",
			Description: "OCP lab MockUps via adapter (libvirt…): Single SNO, or mock-me (SNO + ACM + managed deployments). Building blocks: vHost, OCP-MGMT, OCP-DEPLOY, VyOS, HAP, ACM.",
			Styles:      []string{StyleSingleSNOOCP, StyleACMMultiCluster},
		},
		{
			ID: GenreApplicationDevelopment, Label: "Application Development",
			Description: "Client and full-stack application design MockUps (UI, runtime, dataflow).",
			Styles:      []string{StyleWindowsUI, StyleWebFullStack},
		},
		{
			ID: GenreInfrastructure, Label: "Infrastructure",
			Description: "Hosts, networks, payloads, multi-cloud cost models, and cheapest ARO lab footprints.",
			Styles:      []string{StyleCloudCostModel, StyleAROAzureLab, StyleInfraNodeNetwork},
		},
		{
			ID: GenreContentManagement, Label: "Content Management",
			Description: "Self-serve personal CDN / media realms (cdn-mgr). Golden example: Surfing. Portal+SSO+BYO keys; cheapcloud watches free-tier; mock-me templates for cdn-mgr customers (~$1 index fee, pass-through cloud bill).",
			Styles:      []string{StyleSelfServePersonalCDN, StyleSurfingCdnR2},
		},
	}

	return CatalogResponse{Genres: genres, Styles: styles}
}

// LookupStyle returns a style definition or nil.
func LookupStyle(id string) *StyleDef {
	for _, s := range Catalog().Styles {
		if s.ID == id {
			cp := s
			return &cp
		}
	}
	return nil
}

// ResolveCreateStyle picks genre/style for Create, defaulting to mock-me.
func ResolveCreateStyle(genre, style string) (g, st string, def *StyleDef, err error) {
	if style == "" {
		style = StyleACMMultiCluster
	}
	def = LookupStyle(style)
	if def == nil {
		return "", "", nil, fmt.Errorf("unknown style %q", style)
	}
	if genre == "" {
		genre = def.Genre
	}
	if genre != def.Genre {
		return "", "", nil, fmt.Errorf("style %q belongs to genre %q, not %q", style, def.Genre, genre)
	}
	if !def.Available {
		return "", "", nil, fmt.Errorf("style %q (%s) is not creatable yet — catalog stub", style, def.Label)
	}
	return genre, style, def, nil
}
