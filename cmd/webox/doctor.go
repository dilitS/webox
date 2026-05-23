package main

import (
	"context"
	"fmt"
	"io"

	"github.com/dilitS/webox/services/doctor"
)

// doctorRunner is the narrow seam that production wires to
// [doctor.NewDefault] and tests wire to in-memory stubs. Keeping it as
// an interface (instead of a function value or a package-level var)
// avoids the global-var test-seam trap that previously triggered race
// detector failures in `cmd/webox`.
type doctorRunner interface {
	Run(ctx context.Context) doctor.Report
}

// runDoctor is the production entry point invoked by [Run] when the
// user types `webox doctor [--json]`. It always builds a fresh default
// runner against the live process environment.
func runDoctor(jsonOutput bool, stdout, stderr io.Writer) int {
	return runDoctorWith(jsonOutput, stdout, stderr, doctor.NewDefault())
}

// runDoctorGitHub is the production entry point for
// `webox doctor github [--json]`. The runner is read-only: it inspects
// the local gh CLI install, the secrets backend slot, and (when
// possible) the GitHub rate limit — but never mutates remote state.
func runDoctorGitHub(jsonOutput bool, stdout, stderr io.Writer) int {
	return runDoctorWith(jsonOutput, stdout, stderr, doctor.NewGitHubDefault())
}

// runDoctorWith executes the supplied runner and writes the report to
// stdout. The render error path goes to stderr — JSON consumers parse
// stdout, so polluting it with English diagnostics would break their
// pipelines.
func runDoctorWith(jsonOutput bool, stdout, stderr io.Writer, runner doctorRunner) int {
	report := runner.Run(context.Background())

	var err error
	if jsonOutput {
		err = doctor.WriteJSON(stdout, report)
	} else {
		err = doctor.WriteText(stdout, report, doctor.TextOptions{Color: doctor.ColorEnabled(stdout)})
	}
	if err != nil {
		fmt.Fprintf(stderr, "webox doctor: render report: %v\n", err)
		return exitMisuse
	}

	return doctor.ExitCode(report)
}
