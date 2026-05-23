// Package github contains the minimal GitHub integration used by the
// project wizard: repository creation, deploy keys, Actions secrets,
// workflow dispatch, and latest-run status. The default transport shells
// out to `gh` so authentication stays in the user's GitHub CLI setup;
// the REST transport is a fallback for PAT-backed environments.
package github
