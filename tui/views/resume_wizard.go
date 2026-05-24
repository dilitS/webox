package views

import (
	"fmt"
	"strings"
)

const (
	resumeWizardMinWidth = 80
	resumeWizardMaxWidth = 110
)

// RenderResumeWizard renders the pending-cleanup resume modal.
func RenderResumeWizard(s Screen) string {
	width := clamp(s.Width, resumeWizardMinWidth, resumeWizardMaxWidth)
	form := s.ResumeForm
	rows := []string{
		"♻ [Resume Wizard]",
		"",
		"Webox found pending cleanup work from an interrupted wizard run.",
		"",
		renderKV("Wizard run", fallback(form.WizardID, "unknown")),
		renderKV("Profile", fallback(form.ProfileAlias, "unknown")),
		renderKV("Updated", fallback(form.UpdatedAt, "unknown")),
		"",
		"Remaining cleanup steps:",
	}
	if len(form.StepNames) == 0 {
		rows = append(rows, "  (snapshot failed to load or contains no steps)")
	} else {
		for i := len(form.StepNames) - 1; i >= 0; i-- {
			rows = append(rows, fmt.Sprintf("  - %s", form.StepNames[i]))
		}
	}
	if len(form.Results) > 0 {
		rows = append(rows, "", "Rollback results:")
		for _, result := range form.Results {
			if result.Err != "" {
				rows = append(rows, "  [FAIL] "+result.Name+": "+result.Err)
			} else {
				rows = append(rows, "  [OK]   "+result.Name)
			}
		}
	}
	if form.RollingBack {
		rows = append(rows, "", s.Spinner+" rolling back")
	}
	if form.Discarding {
		rows = append(
			rows,
			"",
			s.Styles.Alert.Render("Discard requires confirmation."),
			"Type: "+form.DiscardPhrase,
			"Input: "+form.ConfirmInput+" _",
		)
	}
	if form.Err != "" {
		rows = append(rows, "", s.Styles.Alert.Render(form.Err))
	}
	rows = append(
		rows,
		"",
		"[ r ] Roll back now   [ k ] Keep and exit   [ d ] Discard snapshot",
	)
	return s.Styles.ActivePanel.Width(width).Render(strings.Join(rows, "\n"))
}
