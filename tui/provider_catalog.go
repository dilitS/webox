package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dilitS/webox/presets"
	"github.com/dilitS/webox/tui/views"
)

// catalogSnapshot builds the view-layer projection of the
// embedded preset registry consumed by [views.RenderProviderCatalog].
//
// Determinism: the registry returned by [presets.Default] is
// process-wide singleton; the snapshot is built fresh per render
// so the operator's cursor / detail toggle move smoothly. Build
// errors during registry init are reflected as `LoadErrors`
// rather than aborting the snapshot — `webox doctor preset` is
// still the canonical diagnostic surface.
func catalogSnapshot(m Model) views.ProviderCatalogSnapshot {
	reg, err := presets.Default()
	if err != nil {
		return views.ProviderCatalogSnapshot{
			LoadErrors: []string{err.Error()},
			CopyHint:   m.catalog.CopyHint,
		}
	}
	if reg == nil {
		return views.ProviderCatalogSnapshot{CopyHint: m.catalog.CopyHint}
	}

	regions := reg.Regions()
	groups := make([]views.ProviderCatalogGroup, 0, len(regions))
	for _, region := range regions {
		members := reg.ByRegion(region)
		rows := make([]views.ProviderCatalogRow, 0, len(members))
		for _, p := range members {
			rows = append(rows, views.ProviderCatalogRow{
				ID:           p.ID,
				DisplayName:  p.DisplayName,
				ProviderType: p.ProviderType,
				Status:       string(p.Status),
				Markets:      append([]string(nil), p.Markets...),
				Badges:       p.CapabilityBadges(),
			})
		}
		groups = append(groups, views.ProviderCatalogGroup{
			Region: region,
			Rows:   rows,
		})
	}

	selected := m.catalog.SelectedID
	if selected == "" {
		// Default to the first row of the first group so the
		// renderer always paints a selection pill — matches the
		// operator's expectation that a freshly opened screen has
		// a focused row to act on.
		if len(groups) > 0 && len(groups[0].Rows) > 0 {
			selected = groups[0].Rows[0].ID
		}
	}

	snap := views.ProviderCatalogSnapshot{
		Groups:     groups,
		SelectedID: selected,
		LoadErrors: catalogLoadErrors(reg.LoadErrors()),
		CopyHint:   m.catalog.CopyHint,
	}
	if m.catalog.ShowDetail && selected != "" {
		if p, err := reg.Get(selected); err == nil {
			snap.Detail = providerCatalogDetail(p)
		}
	}
	return snap
}

// providerCatalogDetail copies the operator-visible fields from
// [presets.Preset] into the view-layer snapshot. The plaintext
// briefing is composed here so the model can hand the same
// string to the clipboard provider when the operator presses `c`.
func providerCatalogDetail(p *presets.Preset) views.ProviderCatalogDetail {
	d := views.ProviderCatalogDetail{
		ID:               p.ID,
		DisplayName:      p.DisplayName,
		Status:           string(p.Status),
		Region:           p.Region(),
		Markets:          append([]string(nil), p.Markets...),
		PanelName:        p.Panel.Name,
		PanelAPI:         string(p.Panel.API),
		PanelAPIPort:     p.Panel.APIPort,
		PanelSSHRequired: p.Panel.SSHRequired,
		NodeRuntime:      p.Capabilities.NodeRuntime,
		RestartMethod:    p.Capabilities.RestartMethod,
		SSLProvider:      p.Capabilities.SSLProvider,
		Badges:           p.CapabilityBadges(),
		DeployPath:       p.Paths.DeployPathTemplate,
		LogPath:          p.Paths.LogPathTemplate,
		EnvPath:          p.Paths.EnvPathTemplate,
		Probes:           append([]string(nil), p.Probes...),
		KnownRisks:       append([]string(nil), p.KnownRisks...),
		Sources:          append([]string(nil), p.Sources...),
		VerifiedFixture:  p.Verified.FixtureDir,
		VerifiedAt:       p.Verified.LastVerifiedAt,
		VerifiedBy:       p.Verified.VerifiedBy,
	}
	for _, e := range p.Capabilities.DatabaseEngines {
		d.DatabaseEngines = append(d.DatabaseEngines, string(e))
	}
	d.BriefingPlainText = composeProviderBriefing(p)
	return d
}

// composeProviderBriefing returns a plain-text summary of the
// preset suitable for pasting into a Slack channel, incident
// post-mortem, or onboarding doc. The format is stable
// (sectioned, line-oriented) so future contributors can grep
// for it in operator notes.
func composeProviderBriefing(p *presets.Preset) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Webox Provider Briefing — %s\n", p.DisplayName)
	fmt.Fprintf(&b, "  id            %s\n", p.ID)
	fmt.Fprintf(&b, "  provider_type %s\n", p.ProviderType)
	fmt.Fprintf(&b, "  status        %s\n", p.Status)
	fmt.Fprintf(&b, "  region        %s\n", p.Region())
	if len(p.Markets) > 0 {
		fmt.Fprintf(&b, "  markets       %s\n", strings.Join(p.Markets, ", "))
	}
	fmt.Fprintf(&b, "\nPanel\n")
	fmt.Fprintf(&b, "  %s (%s", p.Panel.Name, p.Panel.API)
	if p.Panel.APIPort != 0 {
		fmt.Fprintf(&b, ":%d", p.Panel.APIPort)
	}
	fmt.Fprintf(&b, ")\n")
	if p.Panel.SSHRequired {
		fmt.Fprintf(&b, "  ssh required  yes\n")
	}
	fmt.Fprintf(&b, "\nCapabilities\n")
	fmt.Fprintf(&b, "  node runtime    %s\n", p.Capabilities.NodeRuntime)
	fmt.Fprintf(&b, "  restart method  %s\n", p.Capabilities.RestartMethod)
	fmt.Fprintf(&b, "  ssl provider    %s\n", p.Capabilities.SSLProvider)
	if len(p.Capabilities.DatabaseEngines) > 0 {
		engines := make([]string, len(p.Capabilities.DatabaseEngines))
		for i, e := range p.Capabilities.DatabaseEngines {
			engines[i] = string(e)
		}
		fmt.Fprintf(&b, "  databases       %s\n", strings.Join(engines, ", "))
	}
	fmt.Fprintf(&b, "  badges          %s\n", strings.Join(p.CapabilityBadges(), " · "))
	fmt.Fprintf(&b, "\nPaths\n")
	fmt.Fprintf(&b, "  deploy %s\n", p.Paths.DeployPathTemplate)
	fmt.Fprintf(&b, "  log    %s\n", p.Paths.LogPathTemplate)
	if p.Paths.EnvPathTemplate != "" {
		fmt.Fprintf(&b, "  env    %s\n", p.Paths.EnvPathTemplate)
	}
	if len(p.KnownRisks) > 0 {
		fmt.Fprintf(&b, "\nKnown risks\n")
		for _, risk := range p.KnownRisks {
			fmt.Fprintf(&b, "  - %s\n", risk)
		}
	}
	if len(p.Sources) > 0 {
		fmt.Fprintf(&b, "\nSources\n")
		for _, src := range p.Sources {
			fmt.Fprintf(&b, "  - %s\n", src)
		}
	}
	fmt.Fprintf(&b, "\nGenerated by Webox Provider Catalog (read-only).\n")
	return b.String()
}

// catalogLoadErrors flattens the registry's per-file error map
// into a sorted slice the renderer can join into a single hint
// line. Sorted so the displayed order is deterministic across
// reboots.
func catalogLoadErrors(errs map[string]error) []string {
	if len(errs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(errs))
	for k := range errs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(errs))
	for _, k := range keys {
		out = append(out, k+": "+errs[k].Error())
	}
	return out
}

// catalogRows returns the flat list of catalog rows in the same
// order they would appear on screen. Used by the cursor router
// so up/down arrows traverse the catalog without needing region
// awareness in the model.
func catalogRows(snap views.ProviderCatalogSnapshot) []views.ProviderCatalogRow {
	total := 0
	for _, group := range snap.Groups {
		total += len(group.Rows)
	}
	out := make([]views.ProviderCatalogRow, 0, total)
	for _, group := range snap.Groups {
		out = append(out, group.Rows...)
	}
	return out
}

// catalogRowIndex returns the index of `id` in `rows`. Returns
// -1 when no row matches; the cursor router falls back to the
// first row in that case.
func catalogRowIndex(rows []views.ProviderCatalogRow, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}
