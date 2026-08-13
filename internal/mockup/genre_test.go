package mockup

import "testing"

func TestCatalogHasACMStyle(t *testing.T) {
	c := Catalog()
	if len(c.Genres) < 2 {
		t.Fatalf("want genres, got %d", len(c.Genres))
	}
	s := LookupStyle(StyleACMMultiCluster)
	if s == nil || !s.Available {
		t.Fatal("mock-me style must be available")
	}
	sno := LookupStyle(StyleSingleSNOOCP)
	if sno == nil || !sno.Available {
		t.Fatal("single-sno-ocp style must be available")
	}
	if len(s.Relations) < 3 {
		t.Fatalf("want relation rules, got %d", len(s.Relations))
	}
	win := LookupStyle(StyleWindowsUI)
	if win == nil || win.Available {
		t.Fatal("windows-ui should exist as unavailable stub")
	}
	cdn := LookupStyle(StyleSurfingCdnR2)
	if cdn == nil || cdn.Available {
		t.Fatal("surfing-cdn-r2 should exist as unavailable stub")
	}
	if cdn.Genre != GenreContentManagement {
		t.Fatalf("cdn genre: %s", cdn.Genre)
	}
	personal := LookupStyle(StyleSelfServePersonalCDN)
	if personal == nil || personal.Available {
		t.Fatal("self-serve-cloud-personal-cdn should exist as unavailable stub")
	}
	wantTypes := map[string]bool{"CustomerPortal": true, "IdentitySSO": true, "KeyVault": true, "WebHost": true, "SiteCDN": true}
	for _, ot := range personal.ObjectTypes {
		delete(wantTypes, ot)
	}
	if len(wantTypes) > 0 {
		t.Fatalf("personal CDN missing object types: %v", wantTypes)
	}
}

func TestResolveCreateStyle(t *testing.T) {
	g, st, def, err := ResolveCreateStyle("", "")
	if err != nil || g != GenreClusterManagement || st != StyleACMMultiCluster || def == nil {
		t.Fatalf("defaults: g=%s st=%s err=%v", g, st, err)
	}
	_, _, _, err = ResolveCreateStyle(GenreApplicationDevelopment, StyleWindowsUI)
	if err == nil {
		t.Fatal("expected stub style rejected")
	}
	_, _, _, err = ResolveCreateStyle(GenreApplicationDevelopment, StyleACMMultiCluster)
	if err == nil {
		t.Fatal("expected genre mismatch")
	}
}

func TestCreateSetsGenreStyle(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{Name: "rack1"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Genre != GenreClusterManagement || m.Spec.Style != StyleACMMultiCluster {
		t.Fatalf("genre/style: %s / %s", m.Spec.Genre, m.Spec.Style)
	}
}

func TestCreateAROAzureLab(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{
		Name: "aro-lab", Genre: GenreInfrastructure, Style: StyleAROAzureLab,
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Style != StyleAROAzureLab || m.Spec.Provider != "azure" {
		t.Fatalf("aro: style=%s provider=%s", m.Spec.Style, m.Spec.Provider)
	}
	if m.Spec.Canvas == nil || len(m.Spec.Canvas.Orphans) < 6 {
		t.Fatalf("want ARO canvas orphans, got %+v", m.Spec.Canvas)
	}
	kinds := map[string]bool{}
	for _, o := range m.Spec.Canvas.Orphans {
		kinds[o.Kind] = true
	}
	for _, k := range []string{"cloud-vnet", "cloud-aro-cluster", "cloud-aro-master", "cloud-aro-worker", "cloud-aro-spot-worker"} {
		if !kinds[k] {
			t.Fatalf("missing orphan kind %s", k)
		}
	}
	res := ValidateTopology(m)
	if !res.OK {
		t.Fatalf("aro validate: %+v", res)
	}
}

func TestCreateCloudCostModel(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{
		Name: "cloud1", Genre: GenreInfrastructure, Style: StyleCloudCostModel,
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Style != StyleCloudCostModel || m.Spec.CanvasMode != "freeform" {
		t.Fatalf("cloud model: style=%s mode=%s", m.Spec.Style, m.Spec.CanvasMode)
	}
	if m.Status.CheapcloudProductID == "" {
		t.Fatal("expected cheapcloud product id seeded")
	}
	res := ValidateTopology(m)
	if !res.OK {
		t.Fatalf("cloud validate: %+v", res)
	}
}

func TestCreateSingleSNO(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{
		Name: "sno1", Genre: GenreClusterManagement, Style: StyleSingleSNOOCP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Style != StyleSingleSNOOCP || m.Spec.Hub.Label != "OCP-MGMT" {
		t.Fatalf("sno: style=%s hub=%s", m.Spec.Style, m.Spec.Hub.Label)
	}
	if m.Spec.ACM.Enabled || m.Spec.Hub.InstallACM {
		t.Fatal("SNO style should not enable ACM")
	}
	if len(m.Spec.Clusters) != 0 {
		t.Fatalf("want 0 deployments, got %d", len(m.Spec.Clusters))
	}
	res := ValidateTopology(m)
	if !res.OK {
		t.Fatalf("SNO validate: %+v", res)
	}
}
