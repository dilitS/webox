package config

import (
	"errors"
	"testing"
)

func TestMigrate_NilConfig(t *testing.T) {
	t.Parallel()

	got, err := migrate(nil)
	if got != nil {
		t.Errorf("migrate(nil) returned non-nil cfg = %#v", got)
	}
	if !errors.Is(err, errNilConfig) {
		t.Errorf("migrate(nil) err = %v, want errors.Is(_, errNilConfig)", err)
	}
}

func TestMigrate_CurrentVersion_ReturnsUnchanged(t *testing.T) {
	t.Parallel()

	in := &Config{SchemaVersion: Current}
	out, err := migrate(in)
	if err != nil {
		t.Fatalf("migrate(current) err = %v, want nil", err)
	}
	if out != in {
		t.Errorf("migrate(current) returned different pointer; want unchanged")
	}
}

func TestMigrate_LegacyVersion_NoMigratorRegistered(t *testing.T) {
	t.Parallel()

	in := &Config{SchemaVersion: 0}
	got, err := migrate(in)
	if got != nil {
		t.Errorf("migrate(legacy) returned non-nil cfg = %#v", got)
	}
	if !errors.Is(err, errNoMigrator) {
		t.Errorf("migrate(legacy) err = %v, want errors.Is(_, errNoMigrator)", err)
	}
}
