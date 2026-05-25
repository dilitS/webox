package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

const (
	providerCatalogMinWidth = 80
	providerCatalogMaxWidth = 160
	providerCatalogIDColumn = 28
)

// RenderProviderCatalog renders the Sprint 20 TASK-20.2
// read-only browser over the embedded preset registry.
//
// Layout:
//
//	🗂 [Provider Catalog]
//	  Search-as-you-type / preset registry (read-only)
//
//	🇵🇱 Poland
//	  smallhost-devil       VERIFIED   [http] [ssh] [mysql]
//	  ...
//	🌍 Europe / Global / Advanced
//	  ...
//
//	(selection pill on the row the operator is on)
//
//	──────────────────────────────────────────────────────
//	[Detail: <selected preset>]
//	  panel       …
//	  capabilities…
//	  paths       …
//	  probes      …
//	  risks       …
//
//	[c] copy briefing  [Esc] back  [↑/↓] select  [Enter] toggle detail
//
// The renderer is pure: it consumes only `s.Catalog`. The model
// (`tui.Model.providerCatalog`) is responsible for keeping the
// snapshot in sync with the registry and the operator's
// selection cursor.
func RenderProviderCatalog(s Screen) string {
	width := clamp(s.Width, providerCatalogMinWidth, providerCatalogMaxWidth)
	tokens := theme.Default()

	header := dashboardHeader("🗂 [Provider Catalog]", tokens.Primary)
	body := []string{header, "", s.Styles.Muted.Render("Read-only browser over the embedded preset registry. Use this to discover supported hosting providers before running the new-project wizard."), ""}

	if len(s.Catalog.Groups) == 0 {
		empty := []string{
			s.Styles.Muted.Render("Catalog is empty — the embedded registry failed to load. Run `webox doctor preset` for diagnostics."),
			"",
			s.Styles.HelpHints.Render("[esc] back  [q] quit"),
		}
		body = append(body, empty...)
		return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
	}

	for _, group := range s.Catalog.Groups {
		body = append(body, providerCatalogGroupHeading(group.Region, tokens))
		for _, row := range group.Rows {
			body = append(body, providerCatalogRowLine(s, row, tokens))
		}
		body = append(body, "")
	}

	if detail := providerCatalogDetailBlock(s, tokens); detail != "" {
		body = append(body, detail)
	}

	if len(s.Catalog.LoadErrors) > 0 {
		errLine := s.Styles.Muted.Render(fmt.Sprintf("Load errors (%d): %s", len(s.Catalog.LoadErrors), strings.Join(s.Catalog.LoadErrors, " · ")))
		body = append(body, "", errLine)
	}
	if s.Catalog.CopyHint != "" {
		ack := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.Success)).
			Render("✓ " + s.Catalog.CopyHint)
		body = append(body, "", ack)
	}
	footer := s.Styles.HelpHints.Render("[↑/↓] select  [Enter] toggle detail  [c] copy briefing  [Esc] back  [q] quit")
	body = append(body, "", footer)
	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}

func providerCatalogGroupHeading(region string, tokens theme.Theme) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.Accent)).
		Render(providerCatalogRegionEmoji(region) + " " + region)
}

// providerCatalogRegionEmoji is a small visual cue per region
// so operators scanning the catalog can pick out their target
// market without reading every label. Renderer falls back to a
// generic globe when the region is unknown.
func providerCatalogRegionEmoji(region string) string {
	switch region {
	case "Poland":
		return "🇵🇱"
	case "Europe":
		return "🇪🇺"
	case "Global":
		return "🌍"
	case "Advanced":
		return "🛠"
	default:
		return "🌐"
	}
}

func providerCatalogRowLine(s Screen, row ProviderCatalogRow, tokens theme.Theme) string {
	idCell := pad(row.ID, providerCatalogIDColumn)
	statusCell := providerCatalogStatusCell(row.Status, tokens)
	const statusColumn = 11
	statusCell = pad(statusCell, statusColumn)
	badges := strings.Join(row.Badges, " · ")
	if row.ID == s.Catalog.SelectedID {
		pill := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.TextBright)).
			Background(lipgloss.Color(tokens.Primary)).
			Padding(0, 1).
			Render("▶ " + idCell)
		return "  " + pill + "  " + statusCell + "  " + badges
	}
	return "    " + idCell + "  " + statusCell + "  " + s.Styles.Muted.Render(badges)
}

// providerCatalogStatusCell renders the status verb in colour so
// "verified" reads as success, "deprecated" as error, etc.
func providerCatalogStatusCell(status string, tokens theme.Theme) string {
	colour := tokens.TextDim
	switch status {
	case "verified":
		colour = tokens.Success
	case "candidate":
		colour = tokens.Accent
	case "research":
		colour = tokens.Warning
	case "deprecated":
		colour = tokens.Error
	case "community":
		colour = tokens.TextBright
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colour)).
		Render(strings.ToUpper(status))
}

// providerCatalogDetailBlock renders the bottom strip with the
// expanded preset metadata. Empty when the snapshot has no
// detail (e.g. operator just landed on the catalog and has not
// pressed Enter yet).
func providerCatalogDetailBlock(s Screen, tokens theme.Theme) string {
	d := s.Catalog.Detail
	if d.ID == "" {
		return ""
	}
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.Primary)).
		Render(fmt.Sprintf("📖 [Detail: %s]", d.DisplayName))
	rows := []string{
		"────",
		header,
		renderKV("id", d.ID),
		renderKV("status", d.Status),
		renderKV("region", d.Region),
		renderKV("markets", joinNonEmpty(d.Markets, ", ", "—")),
		"",
		renderKV("panel name", d.PanelName),
		renderKV("panel API", joinPanelAPI(d.PanelAPI, d.PanelAPIPort)),
		renderKV("ssh required", boolYesNo(d.PanelSSHRequired)),
		"",
		renderKV("node runtime", d.NodeRuntime),
		renderKV("restart method", d.RestartMethod),
		renderKV("ssl provider", d.SSLProvider),
		renderKV("databases", joinNonEmpty(d.DatabaseEngines, ", ", "none")),
		renderKV("badges", joinNonEmpty(d.Badges, " · ", "—")),
		"",
		renderKV("deploy path", d.DeployPath),
		renderKV("log path", d.LogPath),
	}
	if d.EnvPath != "" {
		rows = append(rows, renderKV("env path", d.EnvPath))
	}
	if len(d.Probes) > 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextDim)).Render("Probes (documentation; not executed in v0.2 baseline):"))
		for _, probe := range d.Probes {
			rows = append(rows, "  - "+probe)
		}
	}
	if len(d.KnownRisks) > 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.Warning)).Render("Known risks:"))
		for _, risk := range d.KnownRisks {
			rows = append(rows, "  - "+risk)
		}
	}
	if len(d.Sources) > 0 {
		rows = append(rows, "", s.Styles.Muted.Render("Sources:"))
		for _, src := range d.Sources {
			rows = append(rows, "  - "+src)
		}
	}
	if d.VerifiedAt != "" || d.VerifiedFixture != "" || d.VerifiedBy != "" {
		rows = append(rows, "", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(tokens.Success)).Render("Verification"))
		if d.VerifiedFixture != "" {
			rows = append(rows, renderKV("fixture", d.VerifiedFixture))
		}
		if d.VerifiedAt != "" {
			rows = append(rows, renderKV("verified at", d.VerifiedAt))
		}
		if d.VerifiedBy != "" {
			rows = append(rows, renderKV("verified by", d.VerifiedBy))
		}
	}
	return strings.Join(rows, "\n")
}

func joinNonEmpty(in []string, sep, fallbackStr string) string {
	if len(in) == 0 {
		return fallbackStr
	}
	return strings.Join(in, sep)
}

func joinPanelAPI(api string, port int) string {
	if port == 0 {
		return api
	}
	return fmt.Sprintf("%s (:%d)", api, port)
}

func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
