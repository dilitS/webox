package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/dilitS/webox/presets"
)

// presetRegistryProvider is the narrow seam through which CLI
// preset commands obtain a *presets.Registry. Production wires it
// to presets.Default(); tests inject a stubbed registry built from
// fstest.MapFS so they never touch the real embedded catalog.
type presetRegistryProvider func() (*presets.Registry, error)

// presetOpts collects the parsed sub-flags accepted by
// `webox doctor preset`. {host, user, port, timeout} are populated
// only when --probe is set; the documentation surface tolerates
// zero values gracefully.
type presetOpts struct {
	id      string
	json    bool
	probe   bool
	host    string
	user    string
	port    int
	timeout time.Duration
}

// runPresetDoctor implements `webox doctor preset` and
// `webox doctor preset --id <id> [--json] [--probe]`.
//
// In v0.2 baseline the command is read-only: it lists presets and
// shows their capability surface, known risks, sources, and probe
// commands as documentation. Live probe execution requires the
// adapter (`webox doctor cpanel` lands in Sprint 17/18) and is
// returned as a stub message here so operators get actionable
// expectations instead of silent success.
func runPresetDoctor(opts presetOpts, stdout, stderr io.Writer, provider presetRegistryProvider) int {
	reg, err := provider()
	if err != nil {
		fmt.Fprintf(stderr, "webox: load preset registry: %v\n", err)
		return exitMisuse
	}

	switch {
	case opts.id != "":
		return showPreset(opts, reg, stdout, stderr)
	default:
		return listPresets(opts, reg, stdout)
	}
}

func listPresets(opts presetOpts, reg *presets.Registry, stdout io.Writer) int {
	all := reg.List()
	if opts.json {
		return writePresetsJSON(stdout, all, reg.LoadErrors())
	}
	return writePresetsText(stdout, reg)
}

func showPreset(opts presetOpts, reg *presets.Registry, stdout, stderr io.Writer) int {
	p, err := reg.Get(opts.id)
	if err != nil {
		fmt.Fprintf(stderr, "webox: %v\n", err)
		return exitMisuse
	}
	if opts.probe {
		if opts.host != "" && opts.user != "" {
			// Live execution path: Sprint 21 TASK-21.4 wires
			// runPresetProbe through the production SSH
			// runner. The dispatcher (probe.go) handles
			// formatting, exit codes, and per-probe context
			// timeouts.
			return runPresetProbe(
				probeOpts{
					id:      opts.id,
					host:    opts.host,
					user:    opts.user,
					port:    opts.port,
					timeout: opts.timeout,
					json:    opts.json,
				},
				stdout, stderr,
				func() (*presets.Registry, error) { return reg, nil },
				newSSHProbeRunner,
			)
		}
		// No --host / --user yet → surface a polite stub so
		// the operator gets actionable expectations instead
		// of silent metadata dumping.
		fmt.Fprintln(stderr, "webox: --probe also requires --host=<HOST> and --user=<USER> to execute against a live panel.")
		fmt.Fprintln(stderr, "webox: showing declarative preset metadata below; supply both flags to run probes.")
	}
	if opts.json {
		return writeShowJSON(stdout, p)
	}
	return writeShowText(stdout, p)
}

func writePresetsText(stdout io.Writer, reg *presets.Registry) int {
	all := reg.List()
	if len(all) == 0 {
		fmt.Fprintln(stdout, "webox preset catalog is empty.")
		return exitOK
	}
	regions := reg.Regions()
	for i, region := range regions {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s\n", region)
		group := reg.ByRegion(region)
		for _, p := range group {
			fmt.Fprintf(
				stdout,
				"  %-32s %-10s [%s]\n",
				p.ID,
				p.Status,
				strings.Join(p.CapabilityBadges(), " · "),
			)
		}
	}
	if errs := reg.LoadErrors(); len(errs) > 0 {
		fmt.Fprintf(stdout, "\nLoad errors (%d): %s\n", len(errs), presets.FormatLoadErrors(errs))
	}
	fmt.Fprintf(stdout, "\n%d preset(s) loaded. Use `webox doctor preset --id <id>` for details.\n", len(all))
	return exitOK
}

func writeShowText(stdout io.Writer, p *presets.Preset) int {
	fmt.Fprintf(stdout, "%s\n", p.DisplayName)
	fmt.Fprintf(stdout, "  id              %s\n", p.ID)
	fmt.Fprintf(stdout, "  provider_type   %s\n", p.ProviderType)
	fmt.Fprintf(stdout, "  status          %s\n", p.Status)
	if len(p.Markets) > 0 {
		fmt.Fprintf(stdout, "  markets         %s\n", strings.Join(p.Markets, ", "))
	}
	fmt.Fprintf(stdout, "  region          %s\n", p.Region())
	fmt.Fprintf(stdout, "\nPanel\n")
	fmt.Fprintf(stdout, "  name            %s\n", p.Panel.Name)
	fmt.Fprintf(stdout, "  api             %s\n", p.Panel.API)
	if p.Panel.APIPort != 0 {
		fmt.Fprintf(stdout, "  api_port        %d\n", p.Panel.APIPort)
	}
	fmt.Fprintf(stdout, "  ssh_required    %t\n", p.Panel.SSHRequired)
	fmt.Fprintf(stdout, "\nCapabilities\n")
	fmt.Fprintf(stdout, "  node_runtime    %s\n", p.Capabilities.NodeRuntime)
	fmt.Fprintf(stdout, "  restart_method  %s\n", p.Capabilities.RestartMethod)
	fmt.Fprintf(stdout, "  ssl_provider    %s\n", p.Capabilities.SSLProvider)
	fmt.Fprintf(stdout, "  databases       %s\n", joinDatabases(p.Capabilities.DatabaseEngines))
	fmt.Fprintf(stdout, "  badges          %s\n", strings.Join(p.CapabilityBadges(), " · "))
	fmt.Fprintf(stdout, "\nPaths (templates)\n")
	fmt.Fprintf(stdout, "  deploy          %s\n", p.Paths.DeployPathTemplate)
	fmt.Fprintf(stdout, "  log             %s\n", p.Paths.LogPathTemplate)
	if p.Paths.EnvPathTemplate != "" {
		fmt.Fprintf(stdout, "  env             %s\n", p.Paths.EnvPathTemplate)
	}
	if len(p.Probes) > 0 {
		fmt.Fprintf(stdout, "\nProbes (documentation; not executed in v0.2 baseline)\n")
		for _, probe := range p.Probes {
			fmt.Fprintf(stdout, "  - %s\n", probe)
		}
	}
	if len(p.KnownRisks) > 0 {
		fmt.Fprintf(stdout, "\nKnown risks\n")
		for _, risk := range p.KnownRisks {
			fmt.Fprintf(stdout, "  - %s\n", risk)
		}
	}
	if len(p.Sources) > 0 {
		fmt.Fprintf(stdout, "\nSources\n")
		for _, src := range p.Sources {
			fmt.Fprintf(stdout, "  - %s\n", src)
		}
	}
	if p.Verified.FixtureDir != "" || p.Verified.LastVerifiedAt != "" || p.Verified.VerifiedBy != "" {
		fmt.Fprintf(stdout, "\nVerification\n")
		if p.Verified.FixtureDir != "" {
			fmt.Fprintf(stdout, "  fixture_dir       %s\n", p.Verified.FixtureDir)
		}
		if p.Verified.LastVerifiedAt != "" {
			fmt.Fprintf(stdout, "  last_verified_at  %s\n", p.Verified.LastVerifiedAt)
		}
		if p.Verified.VerifiedBy != "" {
			fmt.Fprintf(stdout, "  verified_by       %s\n", p.Verified.VerifiedBy)
		}
	}
	return exitOK
}

func joinDatabases(in []presets.DatabaseEngine) string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		out = append(out, string(e))
	}
	return strings.Join(out, ", ")
}

// presetListJSON / presetShowJSON are shaped explicitly so the
// machine-readable output is stable across schema bumps. Keeping
// them separate from presets.Preset means a future schema_version
// 2 won't quietly break --json consumers (e.g. CI scripts).
type presetListJSON struct {
	Presets    []presetListEntry `json:"presets"`
	LoadErrors map[string]string `json:"load_errors,omitempty"`
}

type presetListEntry struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	ProviderType string   `json:"provider_type"`
	Status       string   `json:"status"`
	Region       string   `json:"region"`
	Badges       []string `json:"badges"`
	Markets      []string `json:"markets,omitempty"`
}

func writePresetsJSON(stdout io.Writer, all []*presets.Preset, errs map[string]error) int {
	out := presetListJSON{
		Presets: make([]presetListEntry, 0, len(all)),
	}
	for _, p := range all {
		out.Presets = append(out.Presets, presetListEntry{
			ID:           p.ID,
			DisplayName:  p.DisplayName,
			ProviderType: p.ProviderType,
			Status:       string(p.Status),
			Region:       p.Region(),
			Badges:       p.CapabilityBadges(),
			Markets:      append([]string(nil), p.Markets...),
		})
	}
	if len(errs) > 0 {
		out.LoadErrors = make(map[string]string, len(errs))
		keys := make([]string, 0, len(errs))
		for k := range errs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.LoadErrors[k] = errs[k].Error()
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stdout, "webox: encode preset list json: %v\n", err)
		return exitMisuse
	}
	return exitOK
}

func writeShowJSON(stdout io.Writer, p *presets.Preset) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		fmt.Fprintf(stdout, "webox: encode preset detail json: %v\n", err)
		return exitMisuse
	}
	return exitOK
}
