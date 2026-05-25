package presets_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/dilitS/webox/presets"
)

func mustLoadInMemory(t *testing.T, files map[string]string) *presets.Registry {
	t.Helper()
	mfs := fstest.MapFS{}
	for name, payload := range files {
		mfs["presets/"+name] = &fstest.MapFile{Data: []byte(payload)}
	}
	res, err := presets.LoadFrom(mfs, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	return presets.NewRegistryFromResult(res)
}

func TestRegistryListIsSortedByID(t *testing.T) {
	t.Parallel()

	cpanel := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "cpanel-test"`, 1)
	cpanel = strings.Replace(cpanel, `"provider_type": "smallhost"`, `"provider_type": "cpanel"`, 1)
	cpanel = strings.Replace(cpanel, `"name": "Devil"`, `"name": "cPanel"`, 1)
	cpanel = strings.Replace(cpanel, `"api": "devil_cli"`, `"api": "uapi"`, 1)

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
		"cpanel-test.json":     cpanel,
	})

	got := r.List()
	if len(got) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(got))
	}
	want := []string{"cpanel-test", "smallhost-devil"}
	gotIDs := []string{got[0].ID, got[1].ID}
	if !slices.Equal(gotIDs, want) {
		t.Fatalf("List() ids = %v, want %v", gotIDs, want)
	}
}

func TestRegistryGetReturnsErrPresetNotFound(t *testing.T) {
	t.Parallel()

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
	})

	_, err := r.Get("does-not-exist")
	if !errors.Is(err, presets.ErrPresetNotFound) {
		t.Fatalf("Get() err = %v, want errors.Is(ErrPresetNotFound)", err)
	}
}

func TestRegistryGetReturnsExistingPreset(t *testing.T) {
	t.Parallel()

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
	})

	p, err := r.Get("smallhost-devil")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if p.ID != "smallhost-devil" {
		t.Fatalf("Get(smallhost-devil).ID = %q", p.ID)
	}
}

func TestRegistryByProviderTypeOrdersByStatus(t *testing.T) {
	t.Parallel()

	verified := validPresetMinimal
	research := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "smallhost-research"`, 1)
	research = strings.Replace(research, `"status": "verified"`, `"status": "research"`, 1)
	candidate := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "smallhost-candidate"`, 1)
	candidate = strings.Replace(candidate, `"status": "verified"`, `"status": "candidate"`, 1)
	candidate = strings.Replace(candidate, `"verified": {
    "fixture_dir": "testing/fixtures/smallhost",
    "last_verified_at": "2026-05-25",
    "verified_by": "@maintainer"
  }`, `"verified": {}`, 1)

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-research.json":  research,
		"smallhost-devil.json":     verified,
		"smallhost-candidate.json": candidate,
	})

	got := r.ByProviderType("smallhost")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	wantOrder := []string{"smallhost-devil", "smallhost-candidate", "smallhost-research"}
	for i, p := range got {
		if p.ID != wantOrder[i] {
			t.Fatalf("ByProviderType[%d].ID = %q, want %q (full order: %v)", i, p.ID, wantOrder[i], idsOf(got))
		}
	}
}

func TestRegistryByProviderTypeMissingReturnsEmpty(t *testing.T) {
	t.Parallel()

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
	})
	if got := r.ByProviderType("plesk"); len(got) != 0 {
		t.Fatalf("ByProviderType(plesk) = %v, want empty", got)
	}
}

func TestRegistryByRegionGroupsCorrectly(t *testing.T) {
	t.Parallel()

	usaPreset := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "cpanel-us"`, 1)
	usaPreset = strings.Replace(usaPreset, `"provider_type": "smallhost"`, `"provider_type": "cpanel"`, 1)
	usaPreset = strings.Replace(usaPreset, `"name": "Devil"`, `"name": "cPanel"`, 1)
	usaPreset = strings.Replace(usaPreset, `"api": "devil_cli"`, `"api": "uapi"`, 1)
	usaPreset = strings.Replace(usaPreset, `["PL", "global"]`, `["US"]`, 1)

	dePreset := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "smallhost-de"`, 1)
	dePreset = strings.Replace(dePreset, `["PL", "global"]`, `["DE"]`, 1)

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
		"cpanel-us.json":       usaPreset,
		"smallhost-de.json":    dePreset,
	})

	plGroup := r.ByRegion(presets.RegionPoland)
	if len(plGroup) != 1 || plGroup[0].ID != "smallhost-devil" {
		t.Fatalf("ByRegion(Poland) = %v, want [smallhost-devil]", idsOf(plGroup))
	}
	euGroup := r.ByRegion(presets.RegionEurope)
	if len(euGroup) != 1 || euGroup[0].ID != "smallhost-de" {
		t.Fatalf("ByRegion(Europe) = %v, want [smallhost-de]", idsOf(euGroup))
	}
	glGroup := r.ByRegion(presets.RegionGlobal)
	if len(glGroup) != 1 || glGroup[0].ID != "cpanel-us" {
		t.Fatalf("ByRegion(Global) = %v, want [cpanel-us]", idsOf(glGroup))
	}
}

func TestRegistryRegionsExposesPresentTagsInOrder(t *testing.T) {
	t.Parallel()

	usaPreset := strings.Replace(validPresetMinimal, `"id": "smallhost-devil"`, `"id": "cpanel-us"`, 1)
	usaPreset = strings.Replace(usaPreset, `"provider_type": "smallhost"`, `"provider_type": "cpanel"`, 1)
	usaPreset = strings.Replace(usaPreset, `"name": "Devil"`, `"name": "cPanel"`, 1)
	usaPreset = strings.Replace(usaPreset, `"api": "devil_cli"`, `"api": "uapi"`, 1)
	usaPreset = strings.Replace(usaPreset, `["PL", "global"]`, `["US"]`, 1)

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
		"cpanel-us.json":       usaPreset,
	})

	got := r.Regions()
	want := []string{presets.RegionPoland, presets.RegionGlobal}
	if !slices.Equal(got, want) {
		t.Fatalf("Regions() = %v, want %v", got, want)
	}
}

func TestRegistryLoadErrorsExposesPerFileFailures(t *testing.T) {
	t.Parallel()

	bad := strings.Replace(validPresetMinimal, `"status": "verified"`, `"status": "experimental"`, 1)
	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
		"broken.json":          bad,
	})

	errs := r.LoadErrors()
	if got, want := len(errs), 1; got != want {
		t.Fatalf("len(LoadErrors) = %d, want %d", got, want)
	}
}

func TestRegistryConcurrencySafety(t *testing.T) {
	t.Parallel()

	r := mustLoadInMemory(t, map[string]string{
		"smallhost-devil.json": validPresetMinimal,
	})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_, _ = r.Get("smallhost-devil")
			_ = r.ByProviderType("smallhost")
			_ = r.ByRegion(presets.RegionPoland)
			_ = r.LoadErrors()
			_ = r.Count()
			_ = r.Regions()
		}()
	}
	wg.Wait()
}

func TestDefaultIsCachedSingleton(t *testing.T) {
	// NOT Parallel — Default has process-wide state and other
	// tests in this package may also call Default().

	r1, err := presets.Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if r1 == nil {
		t.Fatal("Default() returned nil registry")
	}
	r2, err := presets.Default()
	if err != nil {
		t.Fatalf("Default() second call error = %v", err)
	}
	if r1 != r2 {
		t.Fatal("Default() returned different pointers across calls; should be a singleton")
	}
	if r1.Count() == 0 {
		t.Fatal("Default().Count() = 0; expected at least the smallhost canonical preset")
	}
}

func idsOf(in []*presets.Preset) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		out = append(out, p.ID)
	}
	return out
}
