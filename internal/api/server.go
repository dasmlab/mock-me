// Package api serves the mock-me UI + MockUp REST API.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/dasmlab/mock-me/internal/activity"
	"github.com/dasmlab/mock-me/internal/auth"
	"github.com/dasmlab/mock-me/internal/cheapcloud"
	"github.com/dasmlab/mock-me/internal/demo"
	"github.com/dasmlab/mock-me/internal/deploy"
	"github.com/dasmlab/mock-me/internal/inventory"
	"github.com/dasmlab/mock-me/internal/mockup"
)

type Server struct {
	store      *mockup.Store
	inventory  *inventory.Store
	activity   *activity.Store
	deploy     *deploy.Engine
	auth       *auth.Service
	cheapcloud *cheapcloud.Client
	dataDir    string
	buildVer   string
	static     http.Handler
	router     chi.Router
}

func New(store *mockup.Store, inv *inventory.Store, act *activity.Store, authSvc *auth.Service, dataDir, buildVer string, static http.Handler) *Server {
	if authSvc == nil {
		authSvc, _ = auth.New(context.Background(), auth.Config{})
	}
	s := &Server{
		store: store, inventory: inv, activity: act, deploy: deploy.NewEngine(store, inv),
		auth: authSvc, cheapcloud: cheapcloud.NewFromEnv(),
		dataDir: dataDir, buildVer: buildVer, static: static,
	}
	if s.activity != nil {
		s.auth.OnLogin = s.recordLogin
	}
	s.router = s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func ListenAndServe(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, h)
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	corsOpts := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	if s.auth != nil && s.auth.Enabled() {
		// Reflect request Origin when SSO cookies are in play (cannot use *).
		corsOpts.AllowOriginFunc = func(_ *http.Request, origin string) bool { return origin != "" }
	} else {
		corsOpts.AllowedOrigins = []string{"*"}
		corsOpts.AllowCredentials = false
	}
	r.Use(cors.Handler(corsOpts))

	r.Get("/healthz", s.healthz)
	r.Get("/isalive", s.healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.healthz)
		r.Get("/version", s.version)
		r.Get("/auth/config", s.authConfig)
		r.Get("/auth/login", s.auth.Login)
		r.Get("/auth/callback", s.auth.Callback)
		r.Get("/auth/logout", s.auth.Logout)
		r.Get("/auth/me", s.auth.Me)
		r.Get("/auth/keepalive", s.auth.KeepAlive)

		// Public demo facade — no Keycloak; never mutates live deploy paths.
		r.Route("/demo", func(r chi.Router) {
			r.Post("/enter", demo.Enter)
			r.Post("/exit", demo.Exit)
			r.Get("/me", demo.Me)
			r.Post("/simulate", demo.Simulate)
			r.Get("/status", demo.Status)
			r.Post("/activity", s.postDemoActivity)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.auth.AdminMiddleware)
			r.Use(demo.DenyMutate(s.auth.IsAdmin))
			r.Get("/profiles", s.profiles)
			r.Get("/catalog", s.catalog)

			r.Get("/mockups", s.listMockups)
			r.Post("/mockups", s.createMockup)
			r.Get("/mockups/{id}", s.getMockup)
			r.Put("/mockups/{id}", s.putMockup)
			r.Patch("/mockups/{id}/layout", s.patchLayout)
			r.Post("/mockups/{id}/clusters", s.addCluster)
			r.Delete("/mockups/{id}/clusters/{clusterId}", s.deleteCluster)
			r.Post("/mockups/{id}/derive", s.derive)
			r.Post("/mockups/{id}/seed-dev-lab", s.seedDevLab)
			r.Post("/mockups/{id}/validate", s.validateMockup)
			r.Post("/mockups/{id}/cost-me", s.costMeMockup)
			r.Post("/mockups/{id}/import-cheapcloud", s.importCheapcloud)
			r.Get("/mockups/{id}/cheapcloud-tracked", s.cheapcloudTracked)
			r.Get("/model/catalog", s.modelCatalog)
			r.Post("/mockups/{id}/deploy", s.deployMockup)
			r.Get("/mockups/{id}/deploy", s.getDeploy)
			r.Post("/mockups/{id}/clean", s.cleanMockup)
			r.Delete("/mockups/{id}", s.deleteMockup)

			r.Get("/inventory", s.listInventory)
			r.Post("/inventory", s.createInventory)
			r.Get("/inventory/{id}", s.getInventory)
			r.Put("/inventory/{id}", s.putInventory)
			r.Post("/inventory/{id}/probe", s.probeInventory)
			r.Post("/inventory/{id}/fix", s.fixInventory)
			r.Delete("/inventory/{id}", s.deleteInventory)

			r.Post("/activity", s.postActivity)
			r.With(s.auth.ActivityViewerGate).Get("/activity", s.listActivity)
		})
	})

	if s.static != nil {
		r.NotFound(s.static.ServeHTTP)
		r.Get("/*", s.static.ServeHTTP)
	}
	return r
}

func (s *Server) authConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.auth.ConfigInfo())
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "warning": "LAB/TEST/DEV ONLY", "version": s.buildVer,
	})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.buildVer})
}

func (s *Server) profiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"hub": []map[string]any{
			{"id": "hub-supported", "cpu": 8, "memoryMiB": 24576, "diskGiB": 200, "unsupported": false},
			{"id": "hub-lab", "cpu": 8, "memoryMiB": 16384, "diskGiB": 160, "unsupported": true},
		},
		"cluster": []map[string]any{
			{"id": "supported", "cpu": 4, "memoryMiB": 16384, "diskGiB": 120, "unsupported": false},
			{"id": "lab-small", "cpu": 4, "memoryMiB": 12288, "diskGiB": 120, "unsupported": true},
		},
	})
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, mockup.Catalog())
}

func (s *Server) listMockups(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*mockup.MockUp{}
	}
	for i := range list {
		list[i] = s.reconcileFailedPhase(list[i])
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createMockup(w http.ResponseWriter, r *http.Request) {
	var req mockup.CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := s.store.Create(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) getMockup(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, s.reconcileFailedPhase(m))
}

// reconcileFailedPhase upgrades validated/deploying → failed when last deploy job failed
// or the status message still shows a mid-deploy stop (older builds left phase at validated).
func (s *Server) reconcileFailedPhase(m *mockup.MockUp) *mockup.MockUp {
	if m == nil || s.deploy == nil {
		return m
	}
	if m.Status.Phase == mockup.PhaseFailed || m.Status.Phase == mockup.PhaseDeployed {
		return m
	}
	msg := m.Status.Message
	job, err := s.deploy.GetJob(m.Metadata.ID)
	jobFailed := err == nil && job != nil && job.Status == "failed"
	msgFailed := strings.Contains(msg, "Stopped at ")
	if !jobFailed && !msgFailed {
		return m
	}
	detail := msg
	if jobFailed && job.Message != "" {
		detail = job.Message
	}
	updated, err := s.store.SetPhase(m.Metadata.ID, mockup.PhaseFailed, detail)
	if err != nil || updated == nil {
		return m
	}
	return updated
}

func (s *Server) putMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var m mockup.MockUp
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.Metadata.ID = existing.Metadata.ID
	m.Metadata.CreatedAt = existing.Metadata.CreatedAt
	if m.APIVersion == "" {
		m.APIVersion = existing.APIVersion
	}
	if m.Kind == "" {
		m.Kind = "MockUp"
	}
	if err := s.store.Save(&m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, &m)
}

func (s *Server) patchLayout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var layout mockup.Layout
	if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.Layout = layout
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) addCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	c := m.AddCluster()
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"cluster": c, "mockup": m})
}

func (s *Server) deleteCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	clusterID := chi.URLParam(r, "clusterId")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := m.RemoveCluster(clusterID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.Save(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) derive(w http.ResponseWriter, r *http.Request) {
	paths, err := s.store.Derive(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paths": paths})
}

func (s *Server) seedDevLab(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.SeedDevLabGaps(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) validateMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	raw = []byte(strings.TrimSpace(string(raw)))
	// Optional JSON body: topology-only teaching check (free-form) without phase advance.
	if len(raw) > 0 && string(raw) != "null" {
		var body mockup.MockUp
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Metadata.ID = id
		writeJSON(w, http.StatusOK, mockup.ValidateTopology(&body))
		return
	}
	res, m, err := s.store.ValidatePlan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               res.OK,
		"mode":             res.Mode,
		"issues":           res.Issues,
		"steps":            res.Steps,
		"summary":          res.Summary,
		"promoteSupported": res.PromoteSupported,
		"mockup":           m,
	})
}

func (s *Server) costMeMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if s.cheapcloud == nil {
		http.Error(w, "cheapcloud client not configured", http.StatusServiceUnavailable)
		return
	}
	register := r.URL.Query().Get("register") == "1" || r.URL.Query().Get("register") == "true"
	req := cheapcloud.Request{
		ProductID:         cheapcloud.ProductID(m),
		MockupID:          m.Metadata.ID,
		RegisterFootprint: register,
		Targets:           cheapcloud.TargetsFromMockUp(m),
	}
	report, err := s.cheapcloud.CostMe(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   err.Error(),
			"targets": req.Targets,
			"url":     s.cheapcloud.BaseURL,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"targets": req.Targets,
		"url":     s.cheapcloud.BaseURL,
		"report":  report,
		"product_id": req.ProductID,
		"mockup":  m,
	})
}

func (s *Server) importCheapcloud(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if s.cheapcloud == nil {
		http.Error(w, "cheapcloud client not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		AttachPolicy string `json:"attach_policy"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	req := cheapcloud.ImportBody(m, body.AttachPolicy)
	res, err := s.cheapcloud.ImportMockUp(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": err.Error(),
			"url":   s.cheapcloud.BaseURL,
			"req":   req,
		})
		return
	}
	m.Status.CheapcloudProductID = req.ProductID
	m.Status.CheapcloudTrackedAt = time.Now().UTC().Format(time.RFC3339)
	m.Metadata.UpdatedAt = m.Status.CheapcloudTrackedAt
	if err := s.store.Save(m); err != nil {
		// fall through — import succeeded remotely
		_ = err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"product_id": req.ProductID,
		"import":     res,
		"url":        s.cheapcloud.BaseURL,
		"mockup":     m,
	})
}

func (s *Server) cheapcloudTracked(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if s.cheapcloud == nil {
		http.Error(w, "cheapcloud client not configured", http.StatusServiceUnavailable)
		return
	}
	tracked, err := s.cheapcloud.TrackedByMockUp(m.Metadata.ID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":      err.Error(),
			"url":        s.cheapcloud.BaseURL,
			"product_id": cheapcloud.ProductID(m),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"product_id": cheapcloud.ProductID(m),
		"url":        s.cheapcloud.BaseURL,
		"tracked":    tracked,
		"mockup":     m,
	})
}

// modelCatalog exposes Design-bench-style cloud palette for the Model UI.
func (s *Server) modelCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"groups": []map[string]any{
			{"id": "network", "label": "Network", "order": 10},
			{"id": "compute", "label": "Compute", "order": 20},
			{"id": "storage", "label": "Storage", "order": 30},
			{"id": "security", "label": "Security", "order": 40},
		},
		"items": []map[string]any{
			{"id": "cloud-vnet", "group": "network", "label": "Virtual network", "kind": "cloud-vnet", "icon": "lan", "defaults": map[string]any{"cidr": "10.42.0.0/16"}},
			{"id": "cloud-subnet", "group": "network", "label": "Subnet", "kind": "cloud-subnet", "icon": "segment", "defaults": map[string]any{"cidr": "10.42.1.0/24"}},
			{"id": "cloud-vm-spot", "group": "compute", "label": "Azure Spot VM", "kind": "cloud-vm-spot", "icon": "bolt", "defaults": map[string]any{"spot": true, "count": 1, "provider": "azure"}},
			{"id": "cloud-aws-ec2-spot", "group": "compute", "label": "AWS EC2 Spot", "kind": "cloud-aws-ec2-spot", "icon": "bolt", "defaults": map[string]any{"spot": true, "sku": "m5.xlarge", "provider": "aws"}},
			{"id": "cloud-gcp-gce-spot", "group": "compute", "label": "GCP GCE Spot", "kind": "cloud-gcp-gce-spot", "icon": "bolt", "defaults": map[string]any{"spot": true, "sku": "n2-standard-4", "provider": "gcp"}},
			{"id": "cloud-ocp-sno-slim", "group": "compute", "label": "OCP SNO slim (Spot)", "kind": "cloud-ocp-sno-slim", "icon": "hub", "defaults": map[string]any{"spot": true, "sku": "Standard_D8s_v3"}},
			{"id": "cloud-aro-cluster", "group": "compute", "label": "ARO cluster", "kind": "cloud-aro-cluster", "icon": "hub", "defaults": map[string]any{"provider": "azure", "region": "eastus"}},
			{"id": "cloud-aro-master", "group": "compute", "label": "ARO masters (×3)", "kind": "cloud-aro-master", "icon": "memory", "defaults": map[string]any{"sku": "Standard_D8s_v3", "count": 3, "spot": false}},
			{"id": "cloud-aro-worker", "group": "compute", "label": "ARO workers (×3)", "kind": "cloud-aro-worker", "icon": "developer_board", "defaults": map[string]any{"sku": "Standard_D4s_v3", "count": 3, "spot": false}},
			{"id": "cloud-aro-spot-worker", "group": "compute", "label": "ARO Spot workers", "kind": "cloud-aro-spot-worker", "icon": "bolt", "defaults": map[string]any{"sku": "Standard_D4s_v3", "count": 2, "spot": true}},
			{"id": "cloud-r2", "group": "storage", "label": "R2 object store", "kind": "cloud-r2", "icon": "cloud", "defaults": map[string]any{"storage_gb": 8}},
			{"id": "cloud-nsg", "group": "security", "label": "Network security group", "kind": "cloud-nsg", "icon": "security", "defaults": map[string]any{"default_deny_ingress": true}},
		},
		"notes": "Multi-cloud palette → Cost me / Import & track. cheapcloud prices + tracks; mock-me models.",
	})
}

func (s *Server) deployMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := existing.RequireUnlocked(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	res, m, err := s.store.ValidatePlan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !res.OK {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":      "validate failed — fix issues before deploy",
			"validation": res,
			"mockup":     m,
		})
		return
	}

	host, reason := s.resolveInventoryHost(m)
	if host == nil {
		http.Error(w, reason, http.StatusConflict)
		return
	}
	m, _ = s.store.LinkInventoryRef(id, host.ID)

	if host.Status != inventory.StatusReachable && s.inventory != nil {
		pr, err := s.inventory.Probe(host.ID)
		if err != nil {
			http.Error(w, "inventory probe failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		host = pr.Host
		if host == nil || host.Status != inventory.StatusReachable {
			msg := "inventory host is not ready (need green Probe)"
			if pr != nil && pr.Message != "" {
				msg = pr.Message
			}
			http.Error(w, msg, http.StatusConflict)
			return
		}
	}

	// Soft preflight: curated EE (openshift-install in container), not host PATH.
	if s.inventory != nil && host != nil {
		ee := strings.TrimSpace(host.Facts["mockMeEE"])
		oi := strings.TrimSpace(host.Facts["openshiftInstall"])
		needProbe := ee == "" || ee == "missing" || ee == "broken" || oi == "missing" || oi == ""
		if needProbe {
			if pr, err := s.inventory.Probe(host.ID); err == nil && pr != nil {
				if pr.Host != nil {
					host = pr.Host
				}
				if !pr.EEReady && !pr.InstallerReady {
					http.Error(w,
						"curated mock-me-ee not ready on inventory host — Probe → Fix this (ensure-mock-me-ee) to pull openshift-install+oc in the EE image",
						http.StatusConflict)
					return
				}
			}
		}
	}

	job, err := s.deploy.Start(id, host.ID, host.Name, host.Endpoint())
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	m, _ = s.store.Get(id)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job":       job,
		"mockup":    m,
		"inventory": host,
		"message":   "Assembly line started — poll GET /mockups/{id}/deploy for stage progress",
	})
}

func (s *Server) getDeploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := s.deploy.GetJob(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "no deploy job", http.StatusNotFound)
		return
	}
	m, _ := s.store.Get(id)
	m = s.reconcileFailedPhase(m)
	writeJSON(w, http.StatusOK, map[string]any{
		"job":    job,
		"mockup": m,
	})
}

func (s *Server) cleanMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Allow Clean when phase was left at validated but job/message still shows failure.
	if m.Status.Phase != mockup.PhaseFailed && m.Status.Phase != mockup.PhaseDeploying {
		m = s.reconcileFailedPhase(m)
	}
	m, err = s.store.CleanFailed(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mockup":  m,
		"message": m.Status.Message,
	})
}

func (s *Server) resolveInventoryHost(m *mockup.MockUp) (*inventory.MachineHost, string) {
	if s.inventory == nil {
		return nil, "inventory store not configured"
	}
	list, err := s.inventory.List()
	if err != nil {
		return nil, "list inventory: " + err.Error()
	}
	if ref := m.Spec.InfraHost.InventoryRef; ref != "" {
		for _, h := range list {
			if h.ID == ref {
				return h, ""
			}
		}
		return nil, "inventoryRef not found: " + ref
	}
	want := strings.TrimSpace(m.Spec.InfraHost.SSHHost)
	for _, h := range list {
		if want != "" && (h.SSHHost == want || h.StretchedHost == want || h.EffectiveSSHHost() == want) {
			return h, ""
		}
	}
	// Fall back to single seed / sole ready host for default click-through.
	var seed *inventory.MachineHost
	var ready *inventory.MachineHost
	for _, h := range list {
		if h.Seed {
			seed = h
		}
		if h.Status == inventory.StatusReachable && ready == nil {
			ready = h
		}
	}
	if ready != nil {
		return ready, ""
	}
	if seed != nil {
		return seed, ""
	}
	if len(list) == 1 {
		return list[0], ""
	}
	return nil, "no inventory MACHINE-HOST to deploy against — add/probe one under Inventory"
}

func (s *Server) deleteMockup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Best-effort: remove libvirt guests + remote work dir for this MockUp name.
	if s.deploy != nil && s.inventory != nil {
		if host, reason := s.resolveInventoryHost(m); host != nil && reason == "" {
			_, _ = s.deploy.TeardownHost(m, host.ID)
		}
	}
	if err := s.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listInventory(w http.ResponseWriter, _ *http.Request) {
	if s.inventory == nil {
		writeJSON(w, http.StatusOK, []*inventory.MachineHost{})
		return
	}
	list, err := s.inventory.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*inventory.MachineHost{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	var req inventory.CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h, err := s.inventory.Create(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) getInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	h, err := s.inventory.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) putInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	existing, err := s.inventory.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var h inventory.MachineHost
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.ID = existing.ID
	h.CreatedAt = existing.CreatedAt
	h.Seed = existing.Seed || h.Seed
	if err := s.inventory.Save(&h); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, &h)
}

func (s *Server) probeInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	res, err := s.inventory.Probe(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) fixInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	var req inventory.FixReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength != 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := s.inventory.Fix(chi.URLParam(r, "id"), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	code := http.StatusOK
	if !res.OK {
		code = http.StatusOK // still 200 with ok:false — UI shows log
	}
	writeJSON(w, code, res)
}

func (s *Server) deleteInventory(w http.ResponseWriter, r *http.Request) {
	if s.inventory == nil {
		http.Error(w, "inventory not configured", http.StatusServiceUnavailable)
		return
	}
	if err := s.inventory.Delete(chi.URLParam(r, "id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) recordLogin(user *auth.User) {
	if s.activity == nil || user == nil {
		return
	}
	_ = s.activity.Append(activity.Event{
		Type:  activity.TypeLogin,
		User:  user.PreferredUsername,
		Sub:   user.Sub,
		Email: user.Email,
	})
}

func (s *Server) postActivity(w http.ResponseWriter, r *http.Request) {
	if s.activity == nil {
		http.Error(w, "activity not configured", http.StatusServiceUnavailable)
		return
	}
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var body struct {
		Type      string `json:"type"`
		Path      string `json:"path"`
		DwellMs   int64  `json:"dwellMs"`
		VisibleMs int64  `json:"visibleMs"`
		EngagedMs int64  `json:"engagedMs"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	typ := strings.TrimSpace(body.Type)
	if typ == "" {
		typ = activity.TypeNavigate
	}
	if typ != activity.TypeNavigate && typ != activity.TypeEngaged {
		http.Error(w, "type must be navigate or engaged", http.StatusBadRequest)
		return
	}
	ev := activity.Event{
		Type:      typ,
		User:      user.PreferredUsername,
		Sub:       user.Sub,
		Email:     user.Email,
		Path:      strings.TrimSpace(body.Path),
		DwellMs:   body.DwellMs,
		VisibleMs: body.VisibleMs,
		EngagedMs: body.EngagedMs,
	}
	if err := s.activity.Append(ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) postDemoActivity(w http.ResponseWriter, r *http.Request) {
	if !demo.IsDemo(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "demo session required"})
		return
	}
	if s.activity == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "skipped": true})
		return
	}
	var body struct {
		Type      string `json:"type"`
		Path      string `json:"path"`
		DwellMs   int64  `json:"dwellMs"`
		VisibleMs int64  `json:"visibleMs"`
		EngagedMs int64  `json:"engagedMs"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	typ := strings.TrimSpace(body.Type)
	if typ == "" {
		typ = activity.TypeNavigate
	}
	ev := activity.Event{
		Type:      typ,
		User:      "demo-visitor",
		Path:      strings.TrimSpace(body.Path),
		DwellMs:   body.DwellMs,
		VisibleMs: body.VisibleMs,
		EngagedMs: body.EngagedMs,
		Demo:      true,
	}
	if err := s.activity.Append(ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "demo": true})
}

func (s *Server) listActivity(w http.ResponseWriter, r *http.Request) {
	if s.activity == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}
	limit := 200
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := s.activity.List(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []activity.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// StaticFS serves an embedded SPA (interview-me / etcd-synthetic-load style).
type StaticFS struct {
	Root http.FileSystem
}

func (s StaticFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Root == nil {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	f, err := s.Root.Open(path)
	if err != nil || isDir(f) {
		if f != nil {
			_ = f.Close()
		}
		f, err = s.Root.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		path = "index.html"
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "static file not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path, stat.ModTime(), rs)
}

func isDir(f http.File) bool {
	if f == nil {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.IsDir()
}
