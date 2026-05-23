package sshmetrics_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dilitS/webox/services/sshmetrics"
	"github.com/dilitS/webox/status"
)

func TestPollHappyPathReturnsParsedMetrics(t *testing.T) {
	t.Parallel()

	runner := sshmetrics.CommandRunnerFunc(func(_ context.Context, _, cmd string) (string, error) {
		switch cmd {
		case "true":
			return "", nil
		case "uptime":
			return " 14:32:01 up 24 days,  3:42,  2 users,  load averages: 0.12, 0.28, 0.31", nil
		case "free -m":
			return "             total       used       free" + "\n" +
				"Mem:          8192       3456       4736", nil
		}
		return "", errors.New("unexpected command")
	})

	poller := sshmetrics.New(runner, nil)
	metrics, _, err := poller.Poll(context.Background(), sshmetrics.Profile{Alias: "main", Host: "demo.example"})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if metrics.Uptime.Days != 24 || metrics.Uptime.LoadAvg1 != 0.12 {
		t.Fatalf("uptime parsing failed: %+v", metrics.Uptime)
	}
	if metrics.Memory.TotalMB != 8192 || metrics.Memory.UsedMB != 3456 {
		t.Fatalf("memory parsing failed: %+v", metrics.Memory)
	}
	if metrics.ProfileAlias != "main" {
		t.Fatalf("alias = %q", metrics.ProfileAlias)
	}
}

func TestPollDegradesMemoryWhenFreeMissing(t *testing.T) {
	t.Parallel()

	runner := sshmetrics.CommandRunnerFunc(func(_ context.Context, _, cmd string) (string, error) {
		switch cmd {
		case "true":
			return "", nil
		case "uptime":
			return " 14:32:01 up 1 day, 5 min,  0 users,  load average: 0.00, 0.00, 0.00", nil
		case "free -m":
			return "", errors.New("free: not found")
		}
		return "", errors.New("unexpected command")
	})

	poller := sshmetrics.New(runner, nil)
	metrics, _, err := poller.Poll(context.Background(), sshmetrics.Profile{Alias: "freebsd"})
	if err != nil {
		t.Fatalf("Poll should degrade not fail, got %v", err)
	}
	if metrics.Memory.TotalMB != 0 {
		t.Fatalf("memory should be zeroed when free missing, got %+v", metrics.Memory)
	}
	if metrics.Uptime.Days != 1 {
		t.Fatalf("uptime should still be parsed: %+v", metrics.Uptime)
	}
}

func TestPollFailsWhenPingCannotConnect(t *testing.T) {
	t.Parallel()

	runner := sshmetrics.CommandRunnerFunc(func(_ context.Context, _, cmd string) (string, error) {
		if cmd == "true" {
			return "", errors.New("dial: refused")
		}
		return "", nil
	})

	poller := sshmetrics.New(runner, nil)
	_, _, err := poller.Poll(context.Background(), sshmetrics.Profile{Alias: "down"})
	if err == nil || !strings.Contains(err.Error(), "ping") {
		t.Fatalf("Poll err = %v, want ping failure", err)
	}
}

func TestPollUsesCacheWithinTTL(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	runner := sshmetrics.CommandRunnerFunc(func(_ context.Context, _, cmd string) (string, error) {
		calls.Add(1)
		switch cmd {
		case "true":
			return "", nil
		case "uptime":
			return " 14:32:01 up 3:14, 1 user, load average: 0.01, 0.02, 0.03", nil
		case "free -m":
			return "Mem: 1024 512 512", nil
		}
		return "", errors.New("unexpected")
	})

	poller := sshmetrics.New(runner, status.NewCache(status.Options{})).
		WithTTL(time.Hour)

	for i := 0; i < 5; i++ {
		if _, _, err := poller.Poll(context.Background(), sshmetrics.Profile{Alias: "cached"}); err != nil {
			t.Fatalf("Poll iter %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("runner called %d times (expected 3 — one per command, then served from cache)", got)
	}
}

func TestNewPanicsOnNilRunner(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil runner")
		}
	}()
	_ = sshmetrics.New(nil, nil)
}
