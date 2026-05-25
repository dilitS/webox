package presets

import "errors"

// Status enumerates the preset lifecycle states defined in
// docs/providers/preconfiguration-vision.md §8.1. The validator
// rejects any other value at load time so callers can branch on
// Status with confidence.
type Status string

const (
	// StatusResearch — public sources only, no test account, no
	// fixtures. Surfaced to operators only behind WEBOX_EXPERIMENTAL.
	StatusResearch Status = "research"
	// StatusCandidate — test account exists, probes pass, fixtures
	// are partial. Suitable for early adopters with explicit
	// confirmation.
	StatusCandidate Status = "candidate"
	// StatusVerified — full fixtures + adapter tests + manual
	// checklist signed off. Default-visible in Provider Catalog.
	StatusVerified Status = "verified"
	// StatusDeprecated — preset no longer reflects reality (panel
	// changed, runtime moved, hoster discontinued the plan). Hidden
	// from Provider Catalog; doctor preset still shows for
	// migration UX.
	StatusDeprecated Status = "deprecated"
	// StatusCommunity — maintained by an external contributor;
	// core team validates schema only.
	StatusCommunity Status = "community"
)

// AllStatuses returns the closed set of valid Status values, in the
// order they appear in the schema. Useful for table-driven tests and
// CLI listing.
func AllStatuses() []Status {
	return []Status{StatusResearch, StatusCandidate, StatusVerified, StatusDeprecated, StatusCommunity}
}

// Valid reports whether s is one of the recognised statuses.
func (s Status) Valid() bool {
	for _, ok := range AllStatuses() {
		if s == ok {
			return true
		}
	}
	return false
}

// PanelAPI enumerates the panel control-plane protocols Webox
// recognises today. The `none` value documents that a preset has no
// panel API at all (pure SSH, e.g. some Devil/small.pl flows).
type PanelAPI string

// Recognised panel control-plane protocols. Schema-validated as an
// enum at load time so callers can branch on these values directly.
const (
	PanelAPIUAPI           PanelAPI = "uapi"
	PanelAPIDirectAdminAPI PanelAPI = "directadmin_api"
	PanelAPICyberPanelCLI  PanelAPI = "cyberpanel_cli"
	PanelAPIDevilCLI       PanelAPI = "devil_cli"
	PanelAPISSHOnly        PanelAPI = "ssh_only"
	PanelAPINone           PanelAPI = "none"
)

// AllPanelAPIs returns the closed set of valid PanelAPI values.
func AllPanelAPIs() []PanelAPI {
	return []PanelAPI{
		PanelAPIUAPI,
		PanelAPIDirectAdminAPI,
		PanelAPICyberPanelCLI,
		PanelAPIDevilCLI,
		PanelAPISSHOnly,
		PanelAPINone,
	}
}

// DatabaseEngine enumerates the database engines a preset may
// declare. Postgres is intentionally listed but rare in shared
// hosting reality; most presets will declare ["mysql"] or
// ["mariadb"].
type DatabaseEngine string

// Recognised database engine values. Schema-validated as an enum
// at load time. `DatabaseNone` lets a preset declare "static-only"
// hosting without bending the required-array constraint.
const (
	DatabaseMySQL      DatabaseEngine = "mysql"
	DatabaseMariaDB    DatabaseEngine = "mariadb"
	DatabasePostgreSQL DatabaseEngine = "postgresql"
	DatabaseSQLite     DatabaseEngine = "sqlite"
	DatabaseNone       DatabaseEngine = "none"
)

// Panel describes the control-plane endpoint of a hosting panel.
type Panel struct {
	Name        string   `json:"name"`
	API         PanelAPI `json:"api"`
	APIPort     int      `json:"api_port,omitempty"`
	SSHRequired bool     `json:"ssh_required"`
}

// Capabilities describes the runtime / restart / SSL / database
// surface a preset declares. Field documentation lives in
// assets/provider-presets/schema.json — keep both in sync (the
// round-trip schema test enforces this).
type Capabilities struct {
	NodeRuntime     string           `json:"node_runtime"`
	RestartMethod   string           `json:"restart_method"`
	SSLProvider     string           `json:"ssl_provider"`
	DatabaseEngines []DatabaseEngine `json:"database_engines"`
	GitAvailable    bool             `json:"git_available,omitempty"`
	SFTPAvailable   bool             `json:"sftp_available,omitempty"`
	LogsPathKnown   bool             `json:"logs_path_known,omitempty"`
	SafeRestart     bool             `json:"safe_restart,omitempty"`
}

// Paths holds filesystem path templates for the preset. Templates
// use {{user}}, {{app_root}}, {{domain}} placeholders that are
// resolved at adapter integration time, not at preset load time.
type Paths struct {
	DeployPathTemplate string `json:"deploy_path_template"`
	LogPathTemplate    string `json:"log_path_template"`
	EnvPathTemplate    string `json:"env_path_template,omitempty"`
	NodeVersionsPath   string `json:"node_versions_path,omitempty"`
}

// Verified holds the verification audit trail for the preset:
// where its fixtures live, when it was last verified, and by whom.
// Empty for status=research.
type Verified struct {
	FixtureDir     string `json:"fixture_dir,omitempty"`
	LastVerifiedAt string `json:"last_verified_at,omitempty"`
	VerifiedBy     string `json:"verified_by,omitempty"`
}

// Preset is the canonical in-memory representation of a provider
// preset. It mirrors assets/provider-presets/schema.json one-to-one
// with the v1 schema; future schema revisions either require a
// non-breaking migration or a new major schema_version + ADR.
type Preset struct {
	SchemaVersion int          `json:"schema_version"`
	ID            string       `json:"id"`
	DisplayName   string       `json:"display_name"`
	ProviderType  string       `json:"provider_type"`
	Status        Status       `json:"status"`
	Markets       []string     `json:"markets,omitempty"`
	Panel         Panel        `json:"panel"`
	Capabilities  Capabilities `json:"capabilities"`
	Paths         Paths        `json:"paths"`
	Probes        []string     `json:"probes,omitempty"`
	KnownRisks    []string     `json:"known_risks,omitempty"`
	Sources       []string     `json:"sources,omitempty"`
	Verified      Verified     `json:"verified,omitempty"`
}

// Region returns a coarse region tag for Provider Catalog grouping.
// The tag is derived from the first market entry: PL → "Poland",
// any other 2-letter code → "Europe" or "Global" depending on the
// country, and "global" → "Global". Presets without markets fall
// into "Advanced".
func (p Preset) Region() string {
	if len(p.Markets) == 0 {
		return RegionAdvanced
	}
	first := p.Markets[0]
	switch first {
	case "PL":
		return RegionPoland
	case "global":
		return RegionGlobal
	}
	if isEuropeanMarket(first) {
		return RegionEurope
	}
	return RegionGlobal
}

// Region tags used by Provider Catalog for grouping. Kept small on
// purpose — the goal is operator clarity, not exhaustive geography.
const (
	RegionPoland   = "Poland"
	RegionEurope   = "Europe"
	RegionGlobal   = "Global"
	RegionAdvanced = "Advanced"
)

// europeanMarkets lists ISO-3166 alpha-2 codes recognised as
// "Europe" for Provider Catalog grouping. Intentionally
// conservative: countries outside this list bucket as "Global"
// rather than guessing.
var europeanMarkets = map[string]struct{}{
	"AT": {}, "BE": {}, "BG": {}, "CH": {}, "CY": {}, "CZ": {}, "DE": {},
	"DK": {}, "EE": {}, "ES": {}, "FI": {}, "FR": {}, "GR": {}, "HR": {},
	"HU": {}, "IE": {}, "IT": {}, "LT": {}, "LU": {}, "LV": {}, "MT": {},
	"NL": {}, "NO": {}, "PT": {}, "RO": {}, "SE": {}, "SI": {}, "SK": {},
	"UA": {}, "UK": {}, "GB": {},
}

func isEuropeanMarket(code string) bool {
	_, ok := europeanMarkets[code]
	return ok
}

// maxCapabilityBadges is the upper bound on labels CapabilityBadges
// may emit (mirrors §9.3 of preconfiguration-vision.md). Used as the
// initial slice capacity so we never reallocate for the production
// case.
const maxCapabilityBadges = 8

// CapabilityBadges returns the capability-badge labels that should
// render next to the preset in Provider Catalog. The order is
// deterministic (matches docs/providers/preconfiguration-vision.md
// §9.3) so snapshot tests stay stable.
func (p Preset) CapabilityBadges() []string {
	badges := make([]string, 0, maxCapabilityBadges)
	if p.Panel.SSHRequired {
		badges = append(badges, "SSH")
	}
	if p.Panel.API != PanelAPINone && p.Panel.API != PanelAPISSHOnly {
		badges = append(badges, "API")
	}
	if p.Capabilities.NodeRuntime != "" && p.Capabilities.NodeRuntime != "static_only" && p.Capabilities.NodeRuntime != "unknown" {
		badges = append(badges, "Node")
	}
	if p.Capabilities.SSLProvider != "" && p.Capabilities.SSLProvider != "none" {
		badges = append(badges, "SSL")
	}
	if hasDatabase(p.Capabilities.DatabaseEngines) {
		badges = append(badges, "DB")
	}
	if p.Capabilities.LogsPathKnown {
		badges = append(badges, "Logs")
	}
	if p.Capabilities.SafeRestart {
		badges = append(badges, "Safe Restart")
	}
	if p.Verified.FixtureDir != "" {
		badges = append(badges, "Fixtures")
	}
	return badges
}

func hasDatabase(engines []DatabaseEngine) bool {
	for _, e := range engines {
		if e != "" && e != DatabaseNone {
			return true
		}
	}
	return false
}

// Errors returned by the loader and validator. Callers branch on
// these via errors.Is so we never compare error.Error() strings.
var (
	// ErrInvalidPreset is the umbrella error for any malformed
	// preset payload. Loader callers usually wrap this with a
	// preset id for context.
	ErrInvalidPreset = errors.New("presets: invalid preset")
	// ErrSchemaViolation indicates the preset JSON did not pass
	// the embedded JSON Schema (Draft 2020-12). The wrapped
	// error contains a flattened, lowercase summary of the
	// failed assertions.
	ErrSchemaViolation = errors.New("presets: schema violation")
	// ErrSecretInPreset indicates the preset contained a string
	// matching one of the well-known secret patterns (GitHub
	// classic/fine-grained tokens, openai-style keys, PEM
	// private-key blocks). Matches must be cleaned up before
	// merge.
	ErrSecretInPreset = errors.New("presets: secret-like token in preset")
	// ErrInvalidJSON indicates the preset payload was not
	// well-formed JSON.
	ErrInvalidJSON = errors.New("presets: invalid json")
	// ErrPresetNotFound indicates the registry has no preset
	// with the requested id. Returned by Registry.Get.
	ErrPresetNotFound = errors.New("presets: preset not found")
	// ErrDuplicateID indicates two embedded presets share the
	// same id. The loader rejects the whole load to avoid
	// silent shadowing.
	ErrDuplicateID = errors.New("presets: duplicate preset id")
)
