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

// RenderInitWizard renders the read-only first-run shell for Sprint 04.
func RenderInitWizard(s Screen) string {
	width := clamp(s.Width, initWizardMinWidth, initWizardMaxWidth)
	body := strings.Join([]string{
		"Step 1/2: System & Agent Environment",
		"",
		s.Styles.Panel.Width(width - initWizardPanelGutter).Render(strings.Join([]string{
			"System Pre-requisites",
			"",
			"Git Engine:       pending doctor check",
			"GitHub CLI (gh):  pending doctor check",
			"Keyring Backend: pending doctor check",
		}, "\n")),
		"",
		s.Styles.Panel.Width(width - initWizardPanelGutter).Render(strings.Join([]string{
			"Default SSH Keypair",
			"",
			"Path: ~/.ssh/id_ed25519_webox",
			"Fingerprint: not generated in Sprint 04",
			"",
			"[ Show Public Key ]     [ Auto-inject to Host ]",
		}, "\n")),
		"",
		"[ Tab ] Navigate   [ Enter ] Confirm   [ Esc ] Quit",
	}, "\n")

	return s.Styles.ActivePanel.
		Width(width).
		Render(fmt.Sprintf("Webox - first run setup\n\n%s", body))
}
