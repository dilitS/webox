package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/internal/version"
	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// View renders the current model without mutating it.
//
// As of Sprint 13 the cockpit composes the visible frame in three
// pinned slots:
//
//  1. Top chrome: a one-line cockpit status bar (`WEBOX vX.Y.Z [LIVE]`
//     + clock / profile / uptime / load / RAM / ping). The dashboard
//     reuses the bento engine's own status bar (composed via
//     [WithStatusBar]) so we do not stack two pills; every other
//     state gets a pinned status bar from [renderChromeTop].
//  2. Body: the active screen's rendered content. When the body is
//     taller than the available height it is sliced through
//     [renderViewport]; PgUp / PgDn / Home / End / mouse-wheel keys
//     scroll only this slot — the bottom footer stays glued to the
//     terminal edge so the operator never loses the keybinding hint
//     while scrolling.
//  3. Bottom chrome: keybinding hint line that surfaces the scroll
//     affordance (`↕ scroll: PgUp/PgDn (offset/max)`) whenever the
//     body overflows the viewport.
//
// Tiny viewports get the raw body (no chrome) so the resize warning
// stays self-contained.
func (m Model) View() string {
	screen := m.screen()
	// Sprint 20 TASK-20.5 — Help overlay is a top-of-stack
	// surface: the operator pressed `?` to ask what they can
	// do *now*, so we replace the entire body with a centered
	// modal. Rendering at View() level (rather than per-state)
	// guarantees the overlay shows up on every state — wizards,
	// catalog, project detail — without each surface needing to
	// opt in.
	if m.helpVisible {
		return helpOverlayFullscreen(m, screen)
	}
	mode := bento.DetectMode(screen.Width, screen.Height)
	if mode == bento.ModeTiny {
		return m.renderRootBody(screen)
	}

	body := m.renderRootBody(screen)
	totalLines := len(viewportLines(body))

	// Dashboard owns its own top status bar via the bento engine, so
	// we keep the body as-is and only pin a footer underneath.
	if m.state == StateDashboard {
		available := screen.Height - 1
		if available < 1 {
			available = 1
		}
		bodyView := renderViewport(body, available, m.viewportOffsetY)
		bottom := m.renderChromeBottom(screen.Width, totalLines, available, m.viewportOffsetY)
		return lipgloss.JoinVertical(lipgloss.Left, bodyView, bottom)
	}

	top := m.renderChromeTop(screen, m.surfaceCrumb())
	available := screen.Height - lipgloss.Height(top) - 1
	if available < 1 {
		available = 1
	}
	bodyView := renderViewport(body, available, m.viewportOffsetY)
	bottom := m.renderChromeBottom(screen.Width, totalLines, available, m.viewportOffsetY)

	return lipgloss.JoinVertical(lipgloss.Left, top, bodyView, bottom)
}

// surfaceCrumb is the per-state breadcrumb the status bar shows
// before the live clock. Each surface owns the answer via its
// [surface.Surface.Crumb] method (Sprint 14 TASK-14.1 finished the
// migration); we delegate here so adding a new surface stays a
// single-file change.
func (m Model) surfaceCrumb() string {
	if s := m.surfaceFor(); s != nil {
		return s.Crumb(surface.Context{Screen: m.screen()})
	}
	return ""
}

// renderRootBody returns the active screen's body (no chrome). Per
// the Sprint 13 chrome contract, top/bottom strips are composed by
// [View]; surfaces only describe their scrollable content.
//
// As of Sprint 14 TASK-14.1 every production state has a
// [surface.Surface] adapter (dashboard, init wizard, project detail,
// project wizard, resume wizard, import preview). The default branch
// remains as a defensive guard: if a future contributor adds a new
// `State` constant without registering a surface in `surfaceFor`,
// the cockpit surfaces a self-explanatory placeholder instead of
// silently rendering an empty body.
func (m Model) renderRootBody(screen views.Screen) string {
	if s := m.surfaceFor(); s != nil {
		return s.Body(surface.Context{Screen: screen})
	}
	return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled", m.state))
}

// renderChromeTop builds the persistent top-of-frame status bar.
// crumb is prefixed to the existing sections so the operator can
// always tell which surface is active.
func (m Model) renderChromeTop(screen views.Screen, crumb string) string {
	opts := m.dashboardStatusBar(screen.Width)
	if crumb != "" {
		opts.Sections = append([]string{crumb}, opts.Sections...)
	}
	return components.RenderStatusBar(opts)
}

// renderChromeBottom builds the pinned footer hint. When the body
// overflows the viewport the footer surfaces the scroll affordance
// inline so the operator does not have to memorise the keymap or
// guess that there is more content below the fold.
//
// Sprint 14 TASK-14.2 — when a tile is focused the footer swaps
// the global "scroll body" hint for a tile-scoped one so the
// operator can tell at a glance that PgUp/PgDn now route inside
// the panel and that Esc releases focus.
//
// Sprint 20 — the global hint is now sourced from the active
// [surface.Surface]'s `Footer().Text` so each state can publish
// keys that actually do something there (e.g. project detail
// surfaces `[1] / [4]` tabs and `[r] / [s] / [v]` actions instead
// of pretending `[/] command palette` exists). When a tile is
// focused the global hint collapses to the absolute-minimum legend
// and the focus suffix takes over the available width — the
// surface's keys are paused (Tab no longer cycles tiles when
// focused; PgUp/PgDn moves the panel; Esc releases). This keeps the
// chrome readable on `120` column terminals where the long
// dashboard hint would otherwise truncate the focus annotation.
func (m Model) renderChromeBottom(width, total, available, offset int) string {
	tokens := theme.Default()
	var hints string
	switch {
	case m.focusedTile != nil:
		hints = defaultGlobalFooterHint
		hints += fmt.Sprintf("  ·  focus: %s · [PgUp/PgDn] scroll panel · [Esc] release",
			slotLabel(*m.focusedTile))
	case total > available && available > 0:
		hints = surfaceFooterText(m.surfaceFor(), m.screen())
		maxOffset := total - available
		hints += fmt.Sprintf("  ·  ↕ scroll: PgUp/PgDn (%d/%d)", offset, maxOffset)
	default:
		hints = surfaceFooterText(m.surfaceFor(), m.screen())
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim)).
		Width(width).
		Render(hints)
}

// surfaceFooterText pulls the per-surface footer hint string. When a
// surface is missing or returns an empty hint we fall back to the
// minimal universal legend so the footer never collapses to an empty
// strip; Tiny terminals never reach this code path because [View]
// short-circuits there.
func surfaceFooterText(s surface.Surface, screen views.Screen) string {
	if s == nil {
		return defaultGlobalFooterHint
	}
	hint := s.Footer(surface.Context{Screen: screen})
	if hint.Text == "" {
		return defaultGlobalFooterHint
	}
	return hint.Text
}

// defaultGlobalFooterHint is the absolute-minimum legend rendered
// when a surface declines to publish its own. Kept short on purpose
// so the right-edge clip on a 100-column terminal still preserves
// the `[q]` and `[?]` cells. Surfaces that need more keys override
// via their [surface.Surface] adapter.
const defaultGlobalFooterHint = "  [q] quit · [?] help"

// slotLabel maps a [bento.Slot] to a short, human-readable label
// for the footer hint. Kept terse so it fits inside the existing
// chrome budget on the 100×30 Standard cockpit fallback.
func slotLabel(slot bento.Slot) string {
	switch slot {
	case bento.SlotProjects:
		return "projects"
	case bento.SlotOverview:
		return "overview"
	case bento.SlotMetrics:
		return "metrics"
	case bento.SlotCICD:
		return "CI/CD"
	case bento.SlotLogs:
		return "logs"
	case bento.SlotTopology:
		return "topology"
	default:
		return "panel"
	}
}

// renderViewport returns the visible slice of rendered for the current
// terminal height. When the full frame fits, the function is a no-op.
// The frame is sliced line-wise (never mid-line), matching how a
// terminal viewport behaves.
func renderViewport(rendered string, height, offset int) string {
	if height <= 0 {
		return rendered
	}
	lines := viewportLines(rendered)
	if len(lines) <= height {
		return strings.Join(lines, "\n")
	}
	start := clampViewportOffset(offset, height, len(lines))
	end := start + height
	return strings.Join(lines[start:end], "\n")
}

func viewportLines(rendered string) []string {
	trimmed := strings.TrimRight(rendered, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func clampViewportOffset(offset, height, total int) int {
	maxOffset := total - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

// maxViewportOffset reports how far the body can be scrolled before
// hitting the bottom. The body excludes the pinned chrome (top status
// bar + bottom footer) which is always 2 lines, matching [View]'s
// composition above.
func (m Model) maxViewportOffset() int {
	const chromeLines = 2
	body := m.renderRootBody(m.screen())
	lines := viewportLines(body)
	available := m.height - chromeLines
	if available < 1 {
		available = 1
	}
	if len(lines) <= available {
		return 0
	}
	return len(lines) - available
}

// renderDashboardBody returns the dashboard body — either the bento
// cockpit (Ultra / Ultra+ / Tiny) or the Standard cockpit two-pane
// fallback. The body owns its own top status bar (bento engine via
// `WithStatusBar`, Standard via a Sprint-13 inline header) so [View]
// can pin only the footer hint around it.
func (m Model) renderDashboardBody(screen views.Screen) string {
	mode := m.BentoMode()
	var base string
	switch mode {
	case bento.ModeStandard:
		statusBar := components.RenderStatusBar(m.dashboardStatusBar(screen.Width))
		body := views.RenderDashboard(screen)
		base = lipgloss.JoinVertical(lipgloss.Left, statusBar, body)
	case bento.ModeTiny:
		base = bento.NewEngine("Webox Cockpit v0.1", nil).
			RenderMode(screen.Width, screen.Height, mode)
	default:
		statusBar := components.RenderStatusBar(m.dashboardStatusBar(screen.Width))
		engine := bento.NewEngine("Webox Cockpit v0.1", m.dashboardBentoTiles()).
			WithStatusBar(statusBar).
			WithTileScrollOffsets(m.tileScrollOffsets)
		if m.focusedTile != nil {
			engine = engine.WithFocus(*m.focusedTile)
		}
		base = engine.RenderMode(screen.Width, screen.Height, mode)
	}
	// Host-key modal takes precedence: the cockpit is unsafe to
	// continue while an SSH dial is being refused; we paint it over
	// anything else so the operator's first interaction is the
	// remediation flow, not the CI/CD logs they were viewing.
	if m.hostKeyModal.Open {
		overlay := renderHostKeyModal(m.hostKeyModal, screen.Width, defaultKnownHostsPath(os.Getenv("HOME")))
		return base + "\n" + overlay
	}
	if !m.cicdModal.Open {
		return base
	}
	overlay := renderCICDLogsModal(m.cicdModal, screen.Width)
	return base + "\n" + overlay
}

// dashboardStatusBar composes the StatusBar snapshot rendered above the
// Bento grid. The snapshot pulls from the active profile + the latest
// SSH metrics cache; missing fields collapse to "—" so the bar never
// shows blank cells.
func (m Model) dashboardStatusBar(width int) components.StatusBarOptions {
	now := m.nowFn()
	tone := components.ToneSuccess
	live := "LIVE"
	stale := m.metricsAreStale()
	if stale {
		tone = components.ToneWarning
		live = "STALE"
	}
	if !m.metricsHaveAnyData() {
		tone = components.ToneInfo
		live = "PENDING"
	}

	sections := []string{now.Format("15:04:05")}
	if profile := m.activeProfileAlias(); profile != "" {
		sections = append(sections, profile)
	}
	if m.headerMetrics.UptimeLabel != "" {
		sections = append(sections, "Uptime: "+m.headerMetrics.UptimeLabel)
	}
	if m.headerMetrics.LoadLabel != "" {
		sections = append(sections, "Load: "+m.headerMetrics.LoadLabel)
	}
	if m.headerMetrics.RAMLabel != "" {
		sections = append(sections, "RAM: "+m.headerMetrics.RAMLabel)
	}
	if m.headerMetrics.RTTLabel != "" {
		sections = append(sections, "Ping: "+m.headerMetrics.RTTLabel)
	}

	return components.StatusBarOptions{
		Brand:     "WEBOX " + version.Short(),
		LiveLabel: live,
		Tone:      tone,
		Sections:  sections,
		Width:     width,
	}
}

// renderCICDLogsModal builds the F8 logs viewer. The modal uses the
// double-border component from Sprint 08 and inherits the FAILED ✗
// red border when the run conclusion was a failure (Sprint 10 plan
// §TASK-10.3 acceptance criteria).
func renderCICDLogsModal(modal cicdLogsModalForm, screenWidth int) string {
	if !modal.Open {
		return ""
	}
	tokens := theme.Default()
	tone := components.ToneInfo
	if modal.RunStatus == bento.CICDStatusFailure {
		tone = components.ToneError
	}

	header := fmt.Sprintf("Workflow Run #%d · %s", modal.RunNumber, cicdModalStatusVerb(modal.RunStatus))
	if modal.ProjectAlias != "" {
		header += " · " + modal.ProjectAlias
	}

	var body strings.Builder
	switch {
	case modal.Loading:
		body.WriteString("Fetching workflow logs (gh run view --log)…")
	case modal.Err != "":
		body.WriteString("Error: ")
		body.WriteString(modal.Err)
	case len(modal.Lines) == 0:
		body.WriteString("No log output yet.")
	default:
		const maxRows = 20
		start := modal.ScrollOffset
		if start < 0 {
			start = 0
		}
		end := start + maxRows
		if end > len(modal.Lines) {
			end = len(modal.Lines)
		}
		for i := start; i < end; i++ {
			line := modal.Lines[i]
			prefix := ""
			if line.StepName != "" {
				prefix = "[" + line.StepName + "] "
			}
			body.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(tokens.TextBright)).
				Render(prefix + line.Text))
			body.WriteString("\n")
		}
		if len(modal.Lines) > maxRows {
			body.WriteString("\n")
			fmt.Fprintf(&body, "(showing %d–%d of %d lines · ↑/↓ scroll)", start+1, end, len(modal.Lines))
		}
	}

	const (
		modalSidePadding = 4
		modalMinWidth    = 60
	)
	minWidth := screenWidth - modalSidePadding
	if minWidth < modalMinWidth {
		minWidth = modalMinWidth
	}

	return components.RenderModal(components.ModalOptions{
		Title:    header,
		Body:     body.String(),
		Footer:   "↑/↓ scroll · Esc/F8 close",
		MinWidth: minWidth,
		Tone:     tone,
		Theme:    tokens,
	})
}

func cicdModalStatusVerb(s bento.CICDStatus) string {
	switch s {
	case bento.CICDStatusSuccess:
		return "SUCCESS ✓"
	case bento.CICDStatusFailure:
		return "FAILED ✗"
	case bento.CICDStatusInProgress:
		return "IN_PROGRESS ⏳"
	case bento.CICDStatusQueued:
		return "QUEUED …"
	case bento.CICDStatusCancelled:
		return "CANCELLED ⊗"
	case bento.CICDStatusSkipped:
		return "SKIPPED ⊘"
	case bento.CICDStatusUnknown:
		return "UNKNOWN ?"
	default:
		return "UNKNOWN ?"
	}
}

func (m Model) dashboardBentoTiles() []bento.BentoTile {
	registry := bento.NewRegistry()
	registry.Register(bento.NewProjectsTile(m.dashboardProjectRows()))
	registry.Register(bento.NewOverviewTile(m.dashboardServerSnapshot()))

	if snap, ok := buildCICDPipelineSnapshot(m); ok {
		registry.Register(bento.NewCICDPipelineTile(snap))
	} else {
		registry.Register(bento.NewCICDPlaceholderTile())
	}
	registry.Register(m.dashboardLiveLogsTile())
	registry.Register(m.dashboardTopologyTile())

	return registry.Tiles()
}

// dashboardTopologyTile builds the live service-topology tile for the
// currently selected project. When the operator has not picked one
// (or there are no projects yet) we fall back to the cyan placeholder
// so the cockpit silhouette never collapses.
func (m Model) dashboardTopologyTile() bento.BentoTile {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return bento.NewTopologyPlaceholderTile()
	}
	project := projects[m.selectedIndex]
	status, hasStatus := m.statuses[project.ID]
	ci, hasCI := m.cicdSnapshots[project.ID]
	// Pulse is driven by the model clock: even seconds → off, odd
	// seconds → on. The bento engine repaints the cockpit on every
	// `RefreshDashboardMsg`, so this naturally shimmers BUILDING /
	// OFFLINE edges without an extra ticker.
	const pulseModulus = 2
	pulse := m.nowFn().Second()%pulseModulus == 1
	snap := buildTopologySnapshot(project, status, hasStatus, ci, hasCI, pulse)
	return bento.NewTopologyTile(snap)
}

func (m Model) dashboardProjectRows() []bento.ProjectRowSnapshot {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 {
		return nil
	}
	rows := make([]bento.ProjectRowSnapshot, 0, len(projects))
	for idx, project := range projects {
		state := ProjectUnknown
		if status, ok := m.statuses[project.ID]; ok {
			state = status.State
		}
		rows = append(rows, bento.ProjectRowSnapshot{
			Name:     project.Domain,
			State:    string(state),
			Selected: idx == m.selectedIndex,
		})
	}
	return rows
}

func (m Model) dashboardServerSnapshot() bento.ServerOverviewSnapshot {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return bento.ServerOverviewSnapshot{ProjectAlias: ""}
	}
	project := projects[m.selectedIndex]
	status, hasStatus := m.statuses[project.ID]

	state := "UNKNOWN"
	if hasStatus {
		state = string(status.State)
	}

	ssl := "unknown"
	sslState := ""
	if hasStatus && status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("Valid (%d days remaining)", status.SSLDaysLeft)
		sslState = "ONLINE"
	} else if hasStatus {
		sslState = "STALE"
	}

	httpHealth := "pending"
	if hasStatus {
		httpHealth = fallbackString(status.HTTPHealth, "pending")
	}
	nodeVer := fallbackString(
		valueIfStatus(hasStatus, status.NodeVersion),
		fallbackString(project.NodeVersion, "unknown"),
	)

	// Icon column intentionally uses 1-cell glyphs (not emoji) so the
	// value column stays vertically aligned. The 2026-05-24 UX
	// refresh routes emoji to the tile *headers* where they sit on
	// their own line; data rows keep the geometric icon set.
	lines := []bento.ServerOverviewLine{
		{Icon: "▣", Label: "Profile", Value: fallbackString(project.ProfileAlias, "(unbound)")},
		{Icon: "◆", Label: "Stack", Value: fallbackString(project.Stack, "—")},
		{Icon: "◉", Label: "Node.js", Value: nodeVer, Status: state},
		{Icon: "✓", Label: "Status", Value: state, Status: state},
		{Icon: "↔", Label: "HTTP", Value: httpHealth},
		{Icon: "⚿", Label: "SSL", Value: ssl, Status: sslState},
		{Icon: "⎇", Label: "Repo", Value: fallbackString(project.Repo, "not linked")},
		{Icon: "⏲", Label: "Last Deploy", Value: fallbackString(valueIfStatus(hasStatus, status.LastDeploy), "—")},
	}
	return bento.ServerOverviewSnapshot{
		ProjectAlias: project.Domain,
		Lines:        lines,
	}
}

// valueIfStatus returns value when hasStatus is true, otherwise the
// empty string. Helper to keep the snapshot builder readable.
func valueIfStatus(hasStatus bool, value string) string {
	if !hasStatus {
		return ""
	}
	return value
}

// dashboardLiveLogsTile renders the bottom-row Live Server Logs tile.
// When no live stream is active (no project selected, or producer not
// yet running), the placeholder fills the slot so the cockpit silhouette
// never collapses.
func (m Model) dashboardLiveLogsTile() bento.BentoTile {
	mode := m.BentoMode()
	tailCap := liveLogsTailCap(mode)
	if m.liveLogs.Buffer == nil || m.liveLogs.Buffer.Len() == 0 {
		return bento.NewLogsPlaceholderTile()
	}
	raw := m.liveLogs.Buffer.Tail(tailCap)
	lines := make([]bento.MicroLogLine, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, bento.MicroLogLine{
			Timestamp: extractTimestamp(line.Text),
			Level:     line.Level,
			Source:    extractSource(line.Text),
			Text:      stripParsedPrefix(line.Text),
			Redacted:  line.Redacted,
		})
	}
	// Subtract the two side gutters reserved by [bento.Engine] when
	// it composes the cockpit grid.
	const sideGutters = 2
	return bento.NewMicroLogsTileWithWidth(m.liveLogs.Domain, lines, m.width-sideGutters)
}

// timestamp / source parsing keeps the cockpit's log tile resilient to
// the mixed log formats SSH `tail -f` emits in the wild. When the
// optional prefix is not present we leave the cells empty so the
// renderer falls back to a level-only row.
//
// The parser is intentionally conservative: missing prefixes never
// raise an error, and the original text is preserved so the operator
// can scroll the raw payload if needed.
func extractTimestamp(text string) string {
	if len(text) >= 10 && text[0] == '[' && text[9] == ']' {
		return text[1:9]
	}
	return ""
}

func extractSource(text string) string {
	idx := strings.Index(text, " - ")
	if idx < 0 {
		return ""
	}
	colon := strings.Index(text[idx+3:], ":")
	if colon < 0 {
		return ""
	}
	return text[idx+3 : idx+3+colon]
}

// stripParsedPrefix returns the message body after `[timestamp] LEVEL - source: `.
// When no prefix is present the original text is returned untouched.
func stripParsedPrefix(text string) string {
	idx := strings.Index(text, ": ")
	if extractTimestamp(text) == "" {
		return text
	}
	if idx < 0 {
		return text
	}
	return text[idx+2:]
}

func fallbackString(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

func (m Model) screen() views.Screen {
	statuses := make(map[string]views.ProjectStatus, len(m.statuses))
	for key, value := range m.statuses {
		statuses[key] = views.ProjectStatus{
			ProjectID:   value.ProjectID,
			HTTPHealth:  value.HTTPHealth,
			SSLDaysLeft: value.SSLDaysLeft,
			NodeVersion: value.NodeVersion,
			LastDeploy:  value.LastDeploy,
			State:       string(value.State),
			Stale:       value.Stale,
		}
	}
	width := m.width
	if width == 0 {
		width = 100
	}
	height := m.height
	if height == 0 {
		height = 30
	}
	return views.Screen{
		Width:         width,
		Height:        height,
		SelectedIndex: m.selectedIndex,
		ActiveTab:     m.activeTab.String(),
		Alert:         m.alert,
		HelpVisible:   m.helpVisible,
		Spinner:       m.spinner.View(),
		Config:        m.cfg,
		Statuses:      statuses,
		Styles:        m.styles,
		InitForm:      initFormSnapshot(m.initForm),
		ProjectForm:   projectFormSnapshot(m.projectForm),
		ResumeForm:    resumeFormSnapshot(m.resumeForm),
		ActionForm:    actionFormSnapshot(m.actionForm),
		ImportForm:    importFormSnapshot(m),
		LiveLogs:      liveLogsSnapshot(m),
		Connections:   dashboardConnectionsSnapshot(m),
		CICDMini:      cicdMiniSnapshot(m),
		Secrets:       secretsSnapshot(m),
		Catalog:       catalogSnapshot(m),
	}
}

// secretsSnapshot turns the [config.Project.SecretsMeta] of the
// currently selected project into the view-layer projection
// consumed by [views.RenderEnvDiff]. Returns nil when no project
// is selected (the Env Diff tab paints a friendly placeholder).
//
// Stale flagging compares `now - LastRotated` against
// `RotationReminderDays`, mirroring the heuristic Webox doctor
// uses on the CLI side. Zero reminder days → never stale.
func secretsSnapshot(m Model) []views.SecretMetaSnapshot {
	project, ok := m.selectedProject()
	if !ok {
		return nil
	}
	if len(project.SecretsMeta) == 0 {
		return nil
	}
	now := m.nowFn()
	const dateLayout = "2006-01-02"
	out := make([]views.SecretMetaSnapshot, 0, len(project.SecretsMeta))
	for _, meta := range project.SecretsMeta {
		snap := views.SecretMetaSnapshot{
			Key:                  meta.Key,
			Source:               string(meta.Source),
			RotationReminderDays: meta.RotationReminderDays,
		}
		if !meta.CreatedAt.IsZero() {
			snap.CreatedAt = meta.CreatedAt.Format(dateLayout)
		}
		if meta.LastRotated != nil && !meta.LastRotated.IsZero() {
			snap.LastRotated = meta.LastRotated.Format(dateLayout)
			if meta.RotationReminderDays > 0 {
				const hoursPerDay = 24
				if int(now.Sub(*meta.LastRotated).Hours()/hoursPerDay) > meta.RotationReminderDays {
					snap.Stale = true
				}
			}
		}
		if meta.LastSyncedGitHub != nil && !meta.LastSyncedGitHub.IsZero() {
			snap.LastSyncedGitHub = meta.LastSyncedGitHub.Format(dateLayout)
		}
		if meta.LastSyncedServer != nil && !meta.LastSyncedServer.IsZero() {
			snap.LastSyncedServer = meta.LastSyncedServer.Format(dateLayout)
		}
		out = append(out, snap)
	}
	return out
}

// cicdMiniSnapshot returns the compact CI/CD projection consumed by
// the Standard cockpit mini-bento strip. Empty when the operator
// has no project selected, no CI run has been observed yet, or
// the snapshot map is uninitialised — the renderer paints the
// `[CI/CD] (no run yet)` placeholder for those branches.
func cicdMiniSnapshot(m Model) views.CICDMiniSnapshot {
	project, ok := m.selectedProject()
	if !ok {
		return views.CICDMiniSnapshot{}
	}
	entry, has := m.cicdSnapshots[project.ID]
	if !has || entry.Run == nil {
		return views.CICDMiniSnapshot{}
	}
	snap := views.CICDMiniSnapshot{
		HasRun:    true,
		Status:    cicdStatusVerb(entry.Run.Status, entry.Run.Conclusion),
		JobName:   entry.Run.Name,
		RunNumber: int64(entry.Run.RunNumber),
	}
	if !entry.Run.HeaderTime.IsZero() {
		snap.UpdatedAt = relativeTime(m.nowFn(), entry.Run.HeaderTime)
	}
	for _, step := range entry.Steps {
		if step.Conclusion == "failure" {
			snap.FailedStep = step.Name
			break
		}
	}
	return snap
}

// cicdStatusVerb maps the GitHub status/conclusion enum pair to the
// upper-case verb the mini-bento ribbon renders. Mirrors
// [bento.CICDStatus] but lives here because the views package
// cannot import bento (cycle).
func cicdStatusVerb(status, conclusion string) string {
	if conclusion != "" {
		switch strings.ToLower(conclusion) {
		case "success":
			return "SUCCESS"
		case "failure":
			return "FAILED"
		case "cancelled":
			return "CANCELLED"
		case "skipped":
			return "SKIPPED"
		}
	}
	switch strings.ToLower(status) {
	case "in_progress":
		return "IN_PROGRESS"
	case "queued":
		return "QUEUED"
	case "completed":
		return "DONE"
	}
	return "UNKNOWN"
}

// relativeTime formats `t` as a human-readable "Nh ago" / "Nm ago"
// string, capped at "1d ago" granularity so the strip stays narrow.
// Future times (clock skew, mock data) collapse to "just now".
func relativeTime(now, t time.Time) string {
	const hoursPerDay = 24
	d := now.Sub(t)
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/hoursPerDay))
	}
}

func dashboardConnectionsSnapshot(m Model) []string {
	project, ok := m.selectedProject()
	if !ok {
		return nil
	}
	status, hasStatus := m.statuses[project.ID]
	ci, hasCI := m.cicdSnapshots[project.ID]
	return buildTopologyConnections(project, status, hasStatus, ci, hasCI)
}

// liveLogsSnapshot is the pure view-layer projection of [liveLogsForm].
// The buffer is read via Snapshot() so consumers cannot mutate the
// streamer's underlying ring while rendering.
func liveLogsSnapshot(m Model) views.LiveLogsSnapshot {
	snap := views.LiveLogsSnapshot{
		Domain:     m.liveLogs.Domain,
		LogPath:    m.liveLogs.LogPath,
		AutoScroll: m.liveLogs.AutoScroll,
		Connected:  m.liveLogs.Connected,
		Err:        m.liveLogs.StreamErr,
	}
	if m.liveLogs.Buffer != nil {
		snap.BufferCap = m.liveLogs.Buffer.Cap()
		snap.BufferUsed = m.liveLogs.Buffer.Len()
		raw := m.liveLogs.Buffer.Tail(liveLogsTailCap(m.BentoMode()))
		for _, line := range raw {
			snap.Lines = append(snap.Lines, views.LiveLogLineSnapshot{
				Level:    line.Level,
				Text:     line.Text,
				Redacted: line.Redacted,
			})
		}
	}
	return snap
}

// Live-log tail-cap thresholds. Numbers come from `docs/UX.md §4.3`
// (Live Log Stream) — Ultra+ shows more history, Standard fits 12 rows
// without scrolling on an 80x24 terminal.
const (
	liveLogsTailCapUltraPlus = 24
	liveLogsTailCapUltra     = 18
	liveLogsTailCapStandard  = 12
)

// liveLogsTailCap caps the number of rendered rows so the live-log
// panel never overflows the cockpit.
func liveLogsTailCap(mode bento.Mode) int {
	switch mode {
	case bento.ModeUltraPlus:
		return liveLogsTailCapUltraPlus
	case bento.ModeUltra:
		return liveLogsTailCapUltra
	default:
		return liveLogsTailCapStandard
	}
}

func importFormSnapshot(m Model) views.ImportPreviewSnapshot {
	snap := views.ImportPreviewSnapshot{
		Loading: m.importForm.Loading,
		Saving:  m.importForm.Saving,
		Err:     m.importForm.Err,
	}
	for _, row := range m.importForm.Rows {
		snap.Rows = append(snap.Rows, views.ImportRowSnapshot{
			ProfileAlias: row.ProfileAlias,
			Domain:       row.Domain,
			Type:         row.Type,
			NodeVersion:  row.NodeVersion,
			Managed:      row.Managed,
		})
		if row.Managed {
			snap.Managed++
		} else {
			snap.Unmanaged++
		}
	}
	snap.Total = len(snap.Rows)
	return snap
}

func actionFormSnapshot(f projectActionForm) views.ProjectActionSnapshot {
	snap := views.ProjectActionSnapshot{
		Kind:      string(f.Kind),
		ProjectID: f.ProjectID,
		Running:   f.Running,
	}
	if f.Output != nil {
		snap.Output = string(f.Output)
	}
	if f.Err != nil {
		snap.Err = f.Err.Error()
	}
	return snap
}

func initFormSnapshot(f initWizardForm) views.InitWizardSnapshot {
	return views.InitWizardSnapshot{
		Step:   int(f.step),
		Alias:  f.alias,
		Host:   f.host,
		Port:   f.port,
		User:   f.user,
		Err:    f.err,
		Saving: f.saving,
	}
}

func projectFormSnapshot(f projectWizardForm) views.ProjectWizardSnapshot {
	snap := views.ProjectWizardSnapshot{
		Step:         int(f.step),
		ProfileAlias: f.profileAlias,
		Stack:        f.stack,
		Domain:       f.domain,
		NodeVersion:  f.nodeVersion,
		DBWanted:     f.dbWanted,
		DBKind:       f.dbKind,
		DBName:       f.dbName,
		Err:          f.err,
		Executing:    f.executing,
		RolledBack:   f.rolledBack,
		RollbackErr:  errString(f.rollbackErr),
	}
	if f.report != nil {
		snap.SubdomainOK = f.report.Subdomain.OK
		snap.SSLOK = f.report.SSL.OK
		snap.DatabaseOK = f.report.Database.OK
		if f.report.Subdomain.Err != nil {
			snap.SubdomainErr = f.report.Subdomain.Err.Error()
		}
		if f.report.SSL.Err != nil {
			snap.SSLErr = f.report.SSL.Err.Error()
		}
		if f.report.Database.Err != nil {
			snap.DatabaseErr = f.report.Database.Err.Error()
		}
	}
	for _, r := range f.rollbackResults {
		snap.RollbackResults = append(snap.RollbackResults, views.RollbackResultSnapshot{
			Name: r.Step.Name,
			Err:  errString(r.Err),
		})
	}
	return snap
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func resumeFormSnapshot(f resumeWizardForm) views.ResumeWizardSnapshot {
	snap := views.ResumeWizardSnapshot{
		Err:           f.err,
		Discarding:    f.discarding,
		DiscardPhrase: f.discardPhrase(),
		ConfirmInput:  f.confirmInput,
		RollingBack:   f.rollingBack,
	}
	if f.snapshot != nil {
		snap.WizardID = f.snapshot.WizardID
		snap.ProfileAlias = f.snapshot.ProfileAlias
		if !f.snapshot.UpdatedAt.IsZero() {
			snap.UpdatedAt = f.snapshot.UpdatedAt.Format("2006-01-02 15:04:05 UTC")
		}
		for _, step := range f.snapshot.Steps {
			snap.StepNames = append(snap.StepNames, step.Name)
		}
	}
	for _, result := range f.results {
		snap.Results = append(snap.Results, views.RollbackResultSnapshot{
			Name: result.Step.Name,
			Err:  errString(result.Err),
		})
	}
	return snap
}
