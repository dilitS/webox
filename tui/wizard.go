package tui

import (
	"strconv"
	"strings"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/wizard"
)

const (
	defaultSSHPort = 22
	maxAliasLength = 32
	maxHostLength  = 253
	maxUserLength  = 32
	maxDomainInput = 253
	maxDBNameInput = 32
	maxStackInput  = 32
	maxPortInput   = 5
)

// initWizardForm holds the in-memory state of the first-run profile
// capture flow. All inputs are strings so the form can render the
// in-progress value without parsing it; conversion to typed values
// happens at submission via [initWizardForm.toProfile].
type initWizardForm struct {
	step   InitWizardStep
	alias  string
	host   string
	port   string
	user   string
	err    string
	saving bool
}

// newInitWizardForm constructs a form ready for the Welcome screen.
// The port pre-fills with the SSH default so the operator can press
// Enter to accept it; tab-back stays meaningful (`22` is a real
// value, not an empty placeholder).
func newInitWizardForm() initWizardForm {
	return initWizardForm{step: InitStepWelcome, port: strconv.Itoa(defaultSSHPort)}
}

// validateAndAdvance moves the form to the next step or sets `err`
// when validation rejects the current input. Pure function: no I/O,
// no globals.
func (f initWizardForm) validateAndAdvance() initWizardForm {
	f.err = ""
	switch f.step {
	case InitStepWelcome:
		f.step = InitStepAlias
	case InitStepAlias:
		alias := strings.TrimSpace(f.alias)
		if !isValidAlias(alias) {
			f.err = "alias must match ^[a-z0-9-]{1,32}$"
			return f
		}
		f.alias = alias
		f.step = InitStepHost
	case InitStepHost:
		host := strings.TrimSpace(f.host)
		if host == "" || len(host) > maxHostLength {
			f.err = "host must be 1-253 characters"
			return f
		}
		f.host = host
		f.step = InitStepPort
	case InitStepPort:
		port := strings.TrimSpace(f.port)
		if port == "" {
			port = strconv.Itoa(defaultSSHPort)
		}
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			f.err = "port must be an integer in [1,65535]"
			return f
		}
		f.port = strconv.Itoa(portNum)
		f.step = InitStepUser
	case InitStepUser:
		user := strings.TrimSpace(f.user)
		if !isValidUser(user) {
			f.err = "user must match ^[a-z0-9_-]{1,32}$"
			return f
		}
		f.user = user
		f.step = InitStepReview
	case InitStepReview:
		f.step = InitStepDone
	}
	return f
}

// stepBack moves the form one step backwards. Welcome is a no-op so
// Shift+Tab on the first screen never escapes the wizard accidentally.
func (f initWizardForm) stepBack() initWizardForm {
	f.err = ""
	switch f.step {
	case InitStepWelcome, InitStepDone:
		return f
	case InitStepAlias:
		f.step = InitStepWelcome
	case InitStepHost:
		f.step = InitStepAlias
	case InitStepPort:
		f.step = InitStepHost
	case InitStepUser:
		f.step = InitStepPort
	case InitStepReview:
		f.step = InitStepUser
	}
	return f
}

// appendInput appends rune to the active step's buffer.
func (f initWizardForm) appendInput(s string) initWizardForm {
	switch f.step {
	case InitStepAlias:
		f.alias = capInput(f.alias+s, maxAliasLength)
	case InitStepHost:
		f.host = capInput(f.host+s, maxHostLength)
	case InitStepPort:
		f.port = capInput(f.port+s, maxPortInput)
	case InitStepUser:
		f.user = capInput(f.user+s, maxUserLength)
	}
	return f
}

// backspaceInput trims the rightmost rune of the active buffer.
func (f initWizardForm) backspaceInput() initWizardForm {
	switch f.step {
	case InitStepAlias:
		f.alias = trimRight(f.alias)
	case InitStepHost:
		f.host = trimRight(f.host)
	case InitStepPort:
		f.port = trimRight(f.port)
	case InitStepUser:
		f.user = trimRight(f.user)
	}
	return f
}

// toProfile converts the form into a [config.Profile]. Validation
// must have already passed; toProfile assumes the strings are clean.
// `type` is hardcoded to "smallhost" — MVP only.
func (f initWizardForm) toProfile() config.Profile {
	port, _ := strconv.Atoi(strings.TrimSpace(f.port))
	if port == 0 {
		port = defaultSSHPort
	}
	return config.Profile{
		Alias:      f.alias,
		Type:       "smallhost",
		Host:       f.host,
		Port:       port,
		User:       f.user,
		Properties: map[string]string{"restart_method": "devil"},
	}
}

// projectWizardForm holds the in-memory state of the new-project
// wizard. Mirrors [wizard.ProvisionPlan] with raw strings so the
// view can render in-progress input.
type projectWizardForm struct {
	step ProjectWizardStep

	profileAlias string
	stack        string
	domain       string
	nodeVersion  string
	dbWanted     bool
	dbKind       string
	dbName       string

	stackCursor   int
	dbKindCursor  int
	profileCursor int

	err           string
	preflightDone bool

	executing bool
	report    *wizard.ProvisionReport
	execErr   error

	rolledBack      bool
	rollbackResults []wizard.CleanupResult
	rollbackErr     error
}

// newProjectWizardForm seeds the form with profile alias + the
// default stack at index 0. The wizard expects len(profiles) > 0 —
// the caller is responsible for routing first-run users to the
// init-wizard instead.
func newProjectWizardForm(cfg *config.Config) projectWizardForm {
	form := projectWizardForm{
		step:   ProjectStepProfile,
		stack:  wizard.SupportedStacks[0],
		dbKind: providers.DatabaseMySQL,
	}
	if cfg != nil && len(cfg.Profiles) > 0 {
		form.profileAlias = cfg.Profiles[0].Alias
	}
	if form.stack != "" {
		form.nodeVersion = wizard.DefaultNodeVersion(form.stack)
		form.dbWanted = wizard.IsDBRequiredForStack(form.stack)
	}
	return form
}

// plan converts the form into a [wizard.ProvisionPlan]. Returns the
// plan as-is — validation runs in [wizard.ValidatePlan] before
// execution. Pure function.
func (f projectWizardForm) plan() wizard.ProvisionPlan {
	plan := wizard.ProvisionPlan{
		ProfileAlias: f.profileAlias,
		Stack:        f.stack,
		Domain:       strings.TrimSpace(f.domain),
		NodeVersion:  strings.TrimSpace(f.nodeVersion),
	}
	if f.dbWanted {
		plan.DBKind = f.dbKind
		plan.DBName = strings.TrimSpace(f.dbName)
	}
	return plan
}

// validateAndAdvance moves the form forwards, applying step-local
// validation. Returns the (possibly updated) form and a bool that
// tells the caller whether the wizard now wants async work (preflight
// + duplicate check after the domain step, execution after review).
func (f projectWizardForm) validateAndAdvance(cfg *config.Config) (projectWizardForm, bool) {
	f.err = ""
	switch f.step {
	case ProjectStepProfile:
		if cfg == nil || len(cfg.Profiles) == 0 {
			f.err = "no profile configured; finish init wizard first"
			return f, false
		}
		if f.profileAlias == "" {
			f.profileAlias = cfg.Profiles[0].Alias
		}
		f.step = ProjectStepStack
	case ProjectStepStack:
		if !wizard.IsValidStack(f.stack) {
			f.err = "unsupported stack"
			return f, false
		}
		f.nodeVersion = wizard.DefaultNodeVersion(f.stack)
		if wizard.IsDBRequiredForStack(f.stack) {
			f.dbWanted = true
			f.step = ProjectStepDBKind
			return f, false
		}
		f.dbWanted = false
		f.step = ProjectStepDBChoice
	case ProjectStepDBChoice:
		if f.dbWanted {
			f.step = ProjectStepDBKind
		} else {
			f.dbName = ""
			f.step = ProjectStepDomain
		}
	case ProjectStepDBKind:
		if !wizard.IsValidDBKind(f.dbKind) {
			f.err = "unsupported db kind"
			return f, false
		}
		f.step = ProjectStepDBName
	case ProjectStepDBName:
		name := strings.TrimSpace(f.dbName)
		if name == "" {
			f.err = "database name is required"
			return f, false
		}
		f.dbName = name
		f.step = ProjectStepDomain
	case ProjectStepDomain:
		domain := strings.TrimSpace(f.domain)
		if domain == "" {
			f.err = "domain is required"
			return f, false
		}
		f.domain = domain
		return f, true
	case ProjectStepReview:
		f.step = ProjectStepExecuting
		f.executing = true
		return f, true
	}
	return f, false
}

// stepBack moves the form one step backwards.
func (f projectWizardForm) stepBack() projectWizardForm {
	f.err = ""
	switch f.step {
	case ProjectStepStack:
		f.step = ProjectStepProfile
	case ProjectStepDBChoice:
		f.step = ProjectStepStack
	case ProjectStepDBKind:
		if wizard.IsDBRequiredForStack(f.stack) {
			f.step = ProjectStepStack
		} else {
			f.step = ProjectStepDBChoice
		}
	case ProjectStepDBName:
		f.step = ProjectStepDBKind
	case ProjectStepDomain:
		if f.dbWanted {
			f.step = ProjectStepDBName
		} else {
			f.step = ProjectStepDBChoice
		}
	case ProjectStepReview:
		f.step = ProjectStepDomain
		f.preflightDone = false
	}
	return f
}

// appendInput appends a typed rune to the active text field.
func (f projectWizardForm) appendInput(s string) projectWizardForm {
	switch f.step {
	case ProjectStepDomain:
		f.domain = capInput(f.domain+s, maxDomainInput)
	case ProjectStepDBName:
		f.dbName = capInput(f.dbName+s, maxDBNameInput)
	}
	return f
}

// backspaceInput trims one rune from the active text field.
func (f projectWizardForm) backspaceInput() projectWizardForm {
	switch f.step {
	case ProjectStepDomain:
		f.domain = trimRight(f.domain)
	case ProjectStepDBName:
		f.dbName = trimRight(f.dbName)
	}
	return f
}

// cycleSelection advances the picker on the stack / db kind / profile
// step. `delta` is +1 for down/right, -1 for up/left.
func (f projectWizardForm) cycleSelection(cfg *config.Config, delta int) projectWizardForm {
	switch f.step {
	case ProjectStepProfile:
		if cfg == nil || len(cfg.Profiles) == 0 {
			return f
		}
		f.profileCursor = wrap(f.profileCursor+delta, len(cfg.Profiles))
		f.profileAlias = cfg.Profiles[f.profileCursor].Alias
	case ProjectStepStack:
		f.stackCursor = wrap(f.stackCursor+delta, len(wizard.SupportedStacks))
		f.stack = wizard.SupportedStacks[f.stackCursor]
	case ProjectStepDBChoice:
		f.dbWanted = !f.dbWanted
	case ProjectStepDBKind:
		f.dbKindCursor = wrap(f.dbKindCursor+delta, len(wizard.SupportedDBKinds))
		f.dbKind = wizard.SupportedDBKinds[f.dbKindCursor]
	}
	return f
}

func capInput(s string, upper int) string {
	if len(s) <= upper {
		return s
	}
	return s[:upper]
}

func trimRight(s string) string {
	if s == "" {
		return s
	}
	return s[:len(s)-1]
}

func wrap(i, n int) int {
	if n <= 0 {
		return 0
	}
	for i < 0 {
		i += n
	}
	return i % n
}

func isValidAlias(alias string) bool {
	if alias == "" || len(alias) > maxAliasLength {
		return false
	}
	for _, r := range alias {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

func isValidUser(user string) bool {
	if user == "" || len(user) > maxUserLength {
		return false
	}
	for _, r := range user {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
