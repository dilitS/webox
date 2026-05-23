package views

import (
	"fmt"
	"strings"
)

const (
	projectWizardMinWidth    = 80
	projectWizardMaxWidth    = 110
	projectWizardPanelGutter = 6
	projectWizardInnerGutter = 4
)

// Project wizard step indices mirror tui.ProjectWizardStep so the
// view package stays free of `tui` imports.
const (
	projectStepProfile = iota
	projectStepStack
	projectStepDBChoice
	projectStepDBKind
	projectStepDBName
	projectStepDomain
	projectStepReview
	projectStepExecuting
	projectStepFailure
	projectStepRollingBack
	projectStepDone
)

// RenderProjectWizard renders the wizard step from the form snapshot.
// Pure function; no allocations besides string concat.
func RenderProjectWizard(s Screen) string {
	width := clamp(s.Width, projectWizardMinWidth, projectWizardMaxWidth)
	body := renderProjectStep(s, width-projectWizardPanelGutter)
	hint := s.Styles.Muted.Render("[ Enter ] Next  [ Shift+Tab ] Back  [ Esc ] Cancel")
	return s.Styles.ActivePanel.
		Width(width).
		Render(fmt.Sprintf("New project wizard\n\n%s\n\n%s", body, hint))
}

func renderProjectStep(s Screen, panelWidth int) string {
	form := s.ProjectForm
	switch form.Step {
	case projectStepStack:
		return renderStackPicker(s, panelWidth, form)
	case projectStepDBChoice:
		return renderDBChoice(s, panelWidth, form)
	case projectStepDBKind:
		return renderDBKindPicker(s, panelWidth, form)
	case projectStepDBName:
		return renderDBName(s, panelWidth, form)
	case projectStepDomain:
		return renderDomain(s, panelWidth, form)
	case projectStepReview:
		return renderProjectReview(s, panelWidth, form)
	case projectStepExecuting:
		return renderProjectExecuting(s, panelWidth, form)
	case projectStepFailure:
		return renderProjectFailure(s, panelWidth, form)
	case projectStepRollingBack:
		return renderRollingBack(s, panelWidth)
	default:
		return renderProfilePicker(s, panelWidth, form)
	}
}

func renderProfilePicker(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	rows := []string{"Step 1/6: Profile", ""}
	if s.Config == nil || len(s.Config.Profiles) == 0 {
		rows = append(rows, s.Styles.Alert.Render("No profile configured."))
		return strings.Join(rows, "\n")
	}
	for _, p := range s.Config.Profiles {
		marker := "  "
		style := s.Styles.ProjectRow
		if p.Alias == form.ProfileAlias {
			marker = "> "
			style = s.Styles.SelectedProjectRow
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s @ %s:%d (%s)", marker, p.Alias, p.Host, p.Port, p.User)))
	}
	rows = append(rows, "",
		s.Styles.Muted.Render("up/down: select"))
	if form.Err != "" {
		rows = append(rows, s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func renderStackPicker(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	options := []string{"static", "vite-react", "node-express"}
	rows := []string{"Step 2/6: Stack", ""}
	for _, o := range options {
		marker := "  "
		style := s.Styles.ProjectRow
		if o == form.Stack {
			marker = "> "
			style = s.Styles.SelectedProjectRow
		}
		rows = append(rows, style.Render(marker+o))
	}
	rows = append(rows, "", s.Styles.Muted.Render("node-express prompts for DB by default."))
	if form.Err != "" {
		rows = append(rows, s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func renderDBChoice(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	choice := "no"
	if form.DBWanted {
		choice = "yes"
	}
	body := []string{
		"Step 3/6: Database (optional)",
		"",
		s.Styles.Muted.Render("Provision a database for this project?"),
		"",
		"Current selection: " + choice,
		"",
		"[ y ] Yes   [ n ] No   [ Enter ] Continue",
	}
	if form.Err != "" {
		body = append(body, "", s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(body, "\n"))
}

func renderDBKindPicker(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	options := []string{"mysql", "postgresql"}
	rows := []string{"Step 3a/6: Database kind", ""}
	for _, o := range options {
		marker := "  "
		style := s.Styles.ProjectRow
		if o == form.DBKind {
			marker = "> "
			style = s.Styles.SelectedProjectRow
		}
		rows = append(rows, style.Render(marker+o))
	}
	if form.Err != "" {
		rows = append(rows, "", s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func renderDBName(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	value := form.DBName
	if value == "" {
		value = s.Styles.Muted.Render("<type here>")
	}
	body := []string{
		"Step 3b/6: Database name",
		"",
		s.Styles.Muted.Render("Lowercase letters / digits / underscore. Max 32 characters."),
		"",
		s.Styles.Panel.Width(panelWidth - projectWizardInnerGutter).Render(value + " _"),
	}
	if form.Err != "" {
		body = append(body, "", s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(body, "\n"))
}

func renderDomain(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	value := form.Domain
	if value == "" {
		value = s.Styles.Muted.Render("<type here>")
	}
	body := []string{
		"Step 4/6: Subdomain",
		"",
		s.Styles.Muted.Render("Fully qualified subdomain, e.g. app.demo.smallhost.pl."),
		"",
		s.Styles.Panel.Width(panelWidth - projectWizardInnerGutter).Render(value + " _"),
	}
	if form.Err != "" {
		body = append(body, "", s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(body, "\n"))
}

func renderProjectReview(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	rows := []string{
		"Step 5/6: Review",
		"",
		s.Styles.Panel.Width(panelWidth - projectWizardInnerGutter).Render(strings.Join([]string{
			"Profile: " + form.ProfileAlias,
			"Stack:   " + form.Stack,
			"Domain:  " + form.Domain,
			"Node:    " + form.NodeVersion,
			"DB:      " + dbSummary(form),
		}, "\n")),
		"",
		s.Styles.Muted.Render("Press Enter to provision; the wizard pushes cleanups to pending_cleanups.json."),
	}
	if form.Err != "" {
		rows = append(rows, "", s.Styles.Alert.Render(form.Err))
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func dbSummary(form ProjectWizardSnapshot) string {
	if !form.DBWanted {
		return "skip"
	}
	return form.DBKind + " / " + form.DBName
}

func renderProjectExecuting(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	rows := []string{
		"Step 6/6: Executing",
		"",
		s.Styles.Muted.Render("Provisioning resources on the panel..."),
		"",
		statusLine("subdomain", form.SubdomainOK, form.SubdomainErr),
		statusLine("ssl", form.SSLOK, form.SSLErr),
		statusLine("database", form.DatabaseOK, form.DatabaseErr),
		"",
		s.Spinner + " in progress",
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func renderProjectFailure(s Screen, panelWidth int, form ProjectWizardSnapshot) string {
	rows := []string{
		s.Styles.Alert.Render("Provisioning failed"),
		"",
		statusLine("subdomain", form.SubdomainOK, form.SubdomainErr),
		statusLine("ssl", form.SSLOK, form.SSLErr),
		statusLine("database", form.DatabaseOK, form.DatabaseErr),
		"",
	}
	if form.Err != "" {
		rows = append(rows, s.Styles.Alert.Render(form.Err), "")
	}
	rows = append(rows,
		"[ y ] Rollback created resources",
		"[ n ] Keep resources and exit",
		"[ Esc ] Exit to dashboard")
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(rows, "\n"))
}

func renderRollingBack(s Screen, panelWidth int) string {
	body := []string{
		"Rolling back...",
		"",
		s.Styles.Muted.Render("Pop + Remove* in reverse push order."),
		"",
		s.Spinner + " in progress",
	}
	return s.Styles.Panel.Width(panelWidth).Render(strings.Join(body, "\n"))
}

func statusLine(name string, ok bool, errMsg string) string {
	switch {
	case errMsg != "":
		return "[FAIL] " + name + ": " + errMsg
	case ok:
		return "[OK]   " + name
	default:
		return "[...]  " + name
	}
}
