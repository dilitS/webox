package providers_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/dilitS/webox/providers"
)

// fullSet returns a freshly-built validator set that satisfies
// IsComplete(). Tests can mutate one field before registering to
// drive a single negative case.
func fullSet() providers.PlanValidators {
	return providers.PlanValidators{
		ValidateDomain:      func(string) error { return nil },
		ValidateNodeVersion: func(string) error { return nil },
		ValidateDBName:      func(string) error { return nil },
	}
}

func TestPlanValidatorsIsComplete(t *testing.T) {
	t.Parallel()

	if !fullSet().IsComplete() {
		t.Fatal("fullSet().IsComplete() = false, want true")
	}

	for _, drop := range []string{"domain", "node", "db"} {
		drop := drop
		t.Run("missing-"+drop, func(t *testing.T) {
			t.Parallel()
			set := fullSet()
			switch drop {
			case "domain":
				set.ValidateDomain = nil
			case "node":
				set.ValidateNodeVersion = nil
			case "db":
				set.ValidateDBName = nil
			}
			if set.IsComplete() {
				t.Fatalf("missing %s but IsComplete = true", drop)
			}
		})
	}
}

func TestRegisterPlanValidatorsRoundtrip(t *testing.T) {
	t.Parallel()

	const name = "test-roundtrip"
	t.Cleanup(func() { providers.UnregisterPlanValidators(name) })

	if err := providers.RegisterPlanValidators(name, fullSet()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := providers.PlanValidatorsFor(name)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !got.IsComplete() {
		t.Fatalf("looked-up set incomplete: %+v", got)
	}
}

func TestRegisterPlanValidatorsRejectsEmptyType(t *testing.T) {
	t.Parallel()

	err := providers.RegisterPlanValidators("   ", fullSet())
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
}

func TestRegisterPlanValidatorsRejectsIncompleteSet(t *testing.T) {
	t.Parallel()

	set := fullSet()
	set.ValidateDBName = nil

	err := providers.RegisterPlanValidators("test-incomplete", set)
	if !errors.Is(err, providers.ErrInvalidValidatorSet) {
		t.Fatalf("err = %v, want ErrInvalidValidatorSet", err)
	}
	if !strings.Contains(err.Error(), "ValidateDBName") {
		t.Fatalf("err = %v, want field-name hint", err)
	}
}

func TestRegisterPlanValidatorsRejectsDuplicate(t *testing.T) {
	t.Parallel()

	const name = "test-duplicate"
	t.Cleanup(func() { providers.UnregisterPlanValidators(name) })

	if err := providers.RegisterPlanValidators(name, fullSet()); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := providers.RegisterPlanValidators(name, fullSet())
	if !errors.Is(err, providers.ErrProviderAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrProviderAlreadyRegistered", err)
	}
}

func TestPlanValidatorsForUnknownReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := providers.PlanValidatorsFor("does-not-exist")
	if !errors.Is(err, providers.ErrUnknownValidator) {
		t.Fatalf("err = %v, want ErrUnknownValidator", err)
	}

	_, err = providers.PlanValidatorsFor("")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("empty err = %v, want ErrInvalidProviderConfig", err)
	}
}

func TestUnregisterPlanValidatorsIdempotent(t *testing.T) {
	t.Parallel()

	const name = "test-unregister"
	if removed := providers.UnregisterPlanValidators(name); removed {
		t.Fatal("unregister of unknown returned true, want false")
	}
	if err := providers.RegisterPlanValidators(name, fullSet()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !providers.UnregisterPlanValidators(name) {
		t.Fatal("first unregister returned false, want true")
	}
	if providers.UnregisterPlanValidators(name) {
		t.Fatal("second unregister returned true, want false (idempotent)")
	}
}

// TestRegistryIsRaceFree exercises the read/write paths concurrently
// to flush out any unsynchronised map access under `-race`. The
// goroutine count is intentionally modest because the registry is
// not a hot path; we just want the race detector signal.
func TestRegistryIsRaceFree(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			name := "test-race-" + string(rune('A'+i%26))
			_ = providers.RegisterPlanValidators(name, fullSet())
			_, _ = providers.PlanValidatorsFor(name)
			_ = providers.UnregisterPlanValidators(name)
		}(i)
	}
	wg.Wait()
}
