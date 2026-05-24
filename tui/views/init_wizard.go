package views

import (
	"fmt"
	"strings"
)

const (
	initWizardMinWidth    = 64
	initWizardMaxWidth    = 100
	initWizardPanelGutter = 6
)

// Init wizard step indices mirror tui.InitWizardStep so the view does
// not import the tui package (cycle). Keep these in sync with
// docs/UX.md §4.1 / §11.1.
const (
	initStepWelcome = iota
	initStepAlias
	initStepHost
	initStepPort
	initStepUser
	initStepReview
	initStepDone
)

// RenderInitWizard renders the read-only first-run shell for the
// init wizard. Step 0 (Welcome) renders the systemic pre-requisites
// silhouette; the form steps render the live input value, helper
// text, and any validation error.
func RenderInitWizard(s Screen) string {
	width := clamp(s.Width, initWizardMinWidth, initWizardMaxWidth)
	body := renderInitStep(s, width-initWizardPanelGutter)

	return s.Styles.ActivePanel.
		Width(width).
		Render(fmt.Sprintf("Webox - first run setup\n\n%s", body))
}

func renderInitStep(s Screen, panelWidth int) string {
	switch s.InitForm.Step {
	case initStepAlias:
		return renderInitField(s, panelWidth, "Profile alias",
			"Step 2/6: pick a short name for this hosting profile (e.g. main, work, prod).",
			s.InitForm.Alias,
			"Must match ^[a-z0-9-]{1,32}$.")
	case initStepHost:
		return renderInitField(s, panelWidth, "SSH host",
			"Step 3/6: hostname of the hosting account (e.g. s1.small.pl).",
			s.InitForm.Host,
			"Use the host you log into via SSH today.")
	case initStepPort:
		return renderInitField(s, panelWidth, "SSH port",
			"Step 4/6: SSH port (default 22).",
			s.InitForm.Port,
			"Integer in [1,65535]; press Enter to accept 22.")
	case initStepUser:
		return renderInitField(s, panelWidth, "SSH user",
			"Step 5/6: SSH account name on the panel.",
			s.InitForm.User,
			"Must match ^[a-z0-9_-]{1,32}$.")
	case initStepReview:
		return renderInitReview(s, panelWidth)
	default:
		return renderInitWelcome(s, panelWidth)
	}
}

// webloxBanner is the ASCII logo painted above the welcome step. We
// keep it compact (≤56 cols) so it never overflows the
// initWizardMinWidth. The block letters use double-line box drawing
// characters that render in every Lipgloss-supported terminal.
const weboxBanner = `╦ ╦╔═╗╔╗ ╔═╗═╗ ╦
║║║║╣ ╠╩╗║ ║╔╩╦╝
╚╩╝╚═╝╚═╝╚═╝╩ ╚═  ·  v0.1 cockpit`

func renderInitWelcome(s Screen, panelWidth int) string {
	banner := s.Styles.Value.Render(weboxBanner)
	lines := []string{
		banner,
		"",
		"Step 1/6: System & Agent Environment",
		"",
		s.Styles.Panel.Width(panelWidth).Render(strings.Join([]string{
			"🛠  System Pre-requisites",
			"",
			" Git Engine:      pending doctor check",
			" GitHub CLI (gh): pending doctor check",
			" Keyring Backend: pending doctor check",
		}, "\n")),
		"",
		s.Styles.Panel.Width(panelWidth).Render(strings.Join([]string{
			"🔑 Default SSH Keypair",
			"",
			" Path:        ~/.ssh/id_ed25519_webox",
			" Fingerprint: deferred to v0.2 (auto-inject flow)",
			"",
			" Webox captures the SSH profile only at this stage;",
			" keypair generation and auto-deploy land in v0.2.",
		}, "\n")),
		"",
		"[ Enter ] Continue   [ Esc ] Quit",
	}
	return strings.Join(lines, "\n")
}

func renderInitField(s Screen, panelWidth int, title, helper, value, hint string) string {
	rendered := value
	if rendered == "" {
		rendered = s.Styles.Muted.Render("<type here>")
	}
	body := []string{
		title,
		"",
		helper,
		"",
		s.Styles.Panel.Width(panelWidth).Render(rendered + " _"),
		"",
		s.Styles.Muted.Render(hint),
	}
	if s.InitForm.Err != "" {
		body = append(body, "", s.Styles.Alert.Render(s.InitForm.Err))
	}
	body = append(body, "", "[ Enter ] Next   [ Shift+Tab ] Back   [ Esc ] Back/Quit")
	return strings.Join(body, "\n")
}

func renderInitReview(s Screen, panelWidth int) string {
	form := s.InitForm
	body := []string{
		"Step 6/6: Review profile",
		"",
		s.Styles.Panel.Width(panelWidth).Render(strings.Join([]string{
			"Alias: " + form.Alias,
			"Type:  smallhost",
			"Host:  " + form.Host,
			"Port:  " + form.Port,
			"User:  " + form.User,
			"Restart: devil",
		}, "\n")),
		"",
		s.Styles.Muted.Render("No secrets stored in config.json (per AGENTS.md §2.1)."),
	}
	if form.Err != "" {
		body = append(body, "", s.Styles.Alert.Render(form.Err))
	}
	if form.Saving {
		body = append(body, "", "Saving profile...")
	} else {
		body = append(body, "", "[ Enter ] Save & continue   [ Shift+Tab ] Back   [ Esc ] Back")
	}
	return strings.Join(body, "\n")
}
