package cheapcloud_test

import (
	"testing"
	"time"

	"github.com/dasmlab/mock-me/internal/cheapcloud"
	"github.com/dasmlab/mock-me/internal/mockup"
)

func TestTargetsFromAROLab(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	m := mockup.LookupStyle(mockup.StyleAROAzureLab)
	if m == nil {
		t.Fatal("style")
	}
	store, err := mockup.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mu, err := store.Create(mockup.CreateReq{
		Name: "aro", Genre: mockup.GenreInfrastructure, Style: mockup.StyleAROAzureLab,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = now
	targets := cheapcloud.TargetsFromMockUp(mu)
	if len(targets) != 1 || targets[0].Capability != "aro-minimal" {
		t.Fatalf("targets=%+v", targets)
	}
	if targets[0].Spot == nil || !*targets[0].Spot || targets[0].Count != 2 {
		t.Fatalf("want spot×2: %+v", targets[0])
	}
}
