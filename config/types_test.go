package config_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/dilitS/webox/config"
)

func TestConfig_RoundTripGoldenFixture(t *testing.T) {
	t.Parallel()

	raw := loadFixture(t, "valid_v1.json")

	var cfg config.Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("decode valid_v1.json into Config: %v", err)
	}

	if got, want := cfg.SchemaVersion, config.Current; got != want {
		t.Errorf("SchemaVersion = %d, want %d", got, want)
	}
	if got, want := cfg.Language, "en"; got != want {
		t.Errorf("Language = %q, want %q", got, want)
	}
	if got, want := len(cfg.Profiles), 1; got != want {
		t.Fatalf("len(Profiles) = %d, want %d", got, want)
	}
	if got, want := cfg.Profiles[0].Alias, "main"; got != want {
		t.Errorf("Profiles[0].Alias = %q, want %q", got, want)
	}
	if got, want := cfg.Profiles[0].Port, 22; got != want {
		t.Errorf("Profiles[0].Port = %d, want %d", got, want)
	}
	if got, want := cfg.Profiles[0].Properties["ssl_mode"], "letsencrypt"; got != want {
		t.Errorf("Profiles[0].Properties[ssl_mode] = %q, want %q", got, want)
	}
	if got, want := len(cfg.Projects), 1; got != want {
		t.Fatalf("len(Projects) = %d, want %d", got, want)
	}
	if got, want := cfg.Projects[0].ProfileAlias, "main"; got != want {
		t.Errorf("Projects[0].ProfileAlias = %q, want %q", got, want)
	}
	if cfg.Projects[0].ImportedAt == nil {
		t.Errorf("Projects[0].ImportedAt is nil, want non-nil")
	} else if got, want := *cfg.Projects[0].ImportedAt, mustTime(t, "2026-05-22T10:30:00Z"); !got.Equal(want) {
		t.Errorf("Projects[0].ImportedAt = %v, want %v", got, want)
	}
	if got, want := len(cfg.Projects[0].SecretsMeta), 1; got != want {
		t.Fatalf("len(Projects[0].SecretsMeta) = %d, want %d", got, want)
	}
	if got, want := cfg.Projects[0].SecretsMeta[0].Source, config.SecretSourceManaged; got != want {
		t.Errorf("SecretsMeta[0].Source = %q, want %q", got, want)
	}
	if cfg.Settings == nil {
		t.Fatal("Settings is nil, want non-nil")
	}
	if got, want := cfg.Settings.RefreshIntervalS, 60; got != want {
		t.Errorf("Settings.RefreshIntervalS = %d, want %d", got, want)
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal Config back to JSON: %v", err)
	}
	if err := config.Validate(encoded); err != nil {
		t.Fatalf("Validate(marshalled Config) = %v, want nil; round-trip broke schema", err)
	}

	var second config.Config
	if err := json.Unmarshal(encoded, &second); err != nil {
		t.Fatalf("decode marshalled Config: %v", err)
	}
	if !reflect.DeepEqual(cfg, second) {
		t.Errorf("round-trip mismatch:\nfirst:  %#v\nsecond: %#v", cfg, second)
	}
}

func TestConfig_NoEmptyInterfaceFields(t *testing.T) {
	t.Parallel()

	rejectInterfaceFields(t, reflect.TypeOf(config.Config{}), "Config")
}

func rejectInterfaceFields(t *testing.T, typ reflect.Type, path string) {
	t.Helper()

	switch typ.Kind() {
	case reflect.Interface:
		t.Errorf("%s is an interface (any/interface{}); strongly-typed fields only", path)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			rejectInterfaceFields(t, f.Type, path+"."+f.Name)
		}
	case reflect.Slice, reflect.Array, reflect.Pointer:
		rejectInterfaceFields(t, typ.Elem(), path+"[]")
	case reflect.Map:
		rejectInterfaceFields(t, typ.Key(), path+"<key>")
		rejectInterfaceFields(t, typ.Elem(), path+"<val>")
	default:
		// scalar — fine
	}
}

func mustTime(t *testing.T, rfc3339 string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t.Fatalf("parse %q: %v", rfc3339, err)
	}
	return parsed
}
