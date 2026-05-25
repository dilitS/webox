package views

import (
	"fmt"
	"strings"
)

// RenderDatabase renders the Sprint 20 TASK-20.4 read-only
// Database tab on the project detail surface.
//
// MVP scope: this view is a STACK-AWARE CHEATSHEET, not a live
// database inspector. Webox v0.1 stores no DB connection state
// in `config.Project` (kind / name / user are passed straight
// to the provider during the wizard and not persisted), so the
// renderer offers operators the next-best thing — a copy-paste
// ready set of connection commands, location of credentials, and
// a pointer to `webox doctor db creds <project>` (Sprint 21).
//
// Why static? Because:
//
//   - Live DB queries from a TUI panel would require an SSH dial
//     per render or a connection pool, both of which violate the
//     pure-MVU contract for `View()`.
//   - The credentials must stay in the keyring / secrets.enc per
//     `docs/SECURITY.md §4`. A "show password" affordance would
//     ship secrets through the cockpit chrome — automatic reject.
//   - Operators on small.pl/Devil already use phpMyAdmin or `psql`
//     for actual DB work; the cockpit's job is to remember the
//     conventions, not become a third-party DB client.
//
// Visual grammar mirrors the Env Diff tab so the four tabs read
// as siblings.
func RenderDatabase(s Screen) string {
	project, ok := selectedProject(s)
	if !ok {
		return s.Styles.Panel.
			Width(clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)).
			Render("No project selected.\n\nEsc: back")
	}
	width := clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)
	tabs := projectDetailTabs(s, "[3] Database")

	header := fmt.Sprintf("🖥 [Project Detail: %s]", project.Domain)

	conventionalDBName := databaseNameForDomain(project.Domain)
	host := databaseHostHint()

	body := []string{
		header,
		"",
		tabs,
		"",
		"💾 [Database] · Stack: " + fallback(project.Stack, "(no stack set)"),
		"",
		s.Styles.Muted.Render("Webox does NOT store DB credentials in config.json. Credentials live in the OS keyring or secrets.enc."),
		"",
		"Conventional database name (Devil panel format):",
		"  " + conventionalDBName,
		"",
		"Connect via:",
		"  " + s.Styles.Muted.Render("# MySQL / MariaDB"),
		"  mysql -h " + host + " -u <devil_user>_<dbuser> -p " + conventionalDBName,
		"",
		"  " + s.Styles.Muted.Render("# PostgreSQL"),
		"  psql -h " + host + " -U <devil_user>_<dbuser> " + conventionalDBName,
		"",
		"Retrieve credentials (Sprint 21+):",
		"  " + s.Styles.Muted.Render("webox doctor db creds "+project.Domain),
		"",
		s.Styles.Muted.Render("Provider docs: https://small.pl/docs/db · GH: dilitS/webox/docs/providers/smallhost.md"),
		s.Styles.HelpHints.Render("[1] overview  [2] env  [4] logs  esc/tab: back"),
	}
	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}

// databaseNameForDomain returns the convention small.pl/Devil
// uses to name databases per project: `<devil_user>_<slug>`,
// where `<slug>` is the leading subdomain. The actual prefix is
// invisible at config time (Webox does not store the panel's
// devil-user), so we render a placeholder the operator can
// substitute. This is documented behaviour, not divination —
// `docs/providers/smallhost.md §3.4` has the full rule set.
func databaseNameForDomain(domain string) string {
	slug := domain
	if idx := strings.Index(domain, "."); idx > 0 {
		slug = domain[:idx]
	}
	slug = strings.ReplaceAll(slug, "-", "_")
	if slug == "" {
		slug = "<project_slug>"
	}
	return "<devil_user>_" + slug
}

// databaseHostHint returns the canonical small.pl/Devil DB host.
// We deliberately do NOT derive the host from `Profile.Alias` —
// aliases are operator-readable labels ("main", "us-east-1"),
// not network endpoints, and Webox v0.1 does not store a
// DB-specific host. Sprint 21+ will route this through
// `providers.HostingProvider` once cPanel ships a sibling
// adapter; until then the cheatsheet uses the documented
// small.pl default.
func databaseHostHint() string {
	return "s1.small.pl"
}
