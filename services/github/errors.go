package github

import "errors"

var (
	// ErrGHUnavailable is returned when the gh CLI is missing or cannot
	// execute. The high-level client may fall back to REST+PAT for this
	// exact failure class.
	ErrGHUnavailable = errors.New("github: gh cli unavailable")

	// ErrRepoExists maps repository conflict / validation responses to a
	// stable sentinel the wizard can treat as recoverable.
	ErrRepoExists = errors.New("github: repository already exists")

	// ErrPATInvalid means the REST fallback token is missing, malformed,
	// expired, or rejected by GitHub.
	ErrPATInvalid = errors.New("github: pat invalid")

	// ErrPATScopeInsufficient means GitHub accepted the token but rejected
	// the operation for permission reasons.
	ErrPATScopeInsufficient = errors.New("github: pat scope insufficient")

	// ErrRateLimited maps primary and secondary rate-limit responses.
	ErrRateLimited = errors.New("github: rate limited")

	// ErrWorkflowDispatchFailed is returned when GitHub refuses to start
	// the first deploy workflow.
	ErrWorkflowDispatchFailed = errors.New("github: workflow dispatch failed")

	// ErrInvalidRepoRef means a repo owner/name pair is incomplete.
	ErrInvalidRepoRef = errors.New("github: invalid repository reference")
	// ErrWorkflowFileMissingSHA means GitHub Contents API returned no SHA.
	ErrWorkflowFileMissingSHA = errors.New("github: workflow file sha missing")
	// ErrInvalidCommitFileRequest means a contents commit lacks path/content.
	ErrInvalidCommitFileRequest = errors.New("github: invalid commit file request")
	// ErrRunGetterNil means a poll loop was started without a run getter.
	ErrRunGetterNil = errors.New("github: run getter is nil")
	// ErrHTTPValidationFailed maps GitHub 422 responses not covered by sentinels.
	ErrHTTPValidationFailed = errors.New("github: validation failed")
	// ErrHTTPServerError maps retryable 5xx responses.
	ErrHTTPServerError = errors.New("github: server error")
	// ErrHTTPUnexpectedStatus maps non-retryable responses without a narrower sentinel.
	ErrHTTPUnexpectedStatus = errors.New("github: unexpected http status")
	// ErrInvalidPublicKey means an Actions secret encryption key is malformed.
	ErrInvalidPublicKey = errors.New("github: invalid actions public key")

	// ErrRunNotFound is returned when the requested workflow run does
	// not exist (HTTP 404 from GitHub or empty jobs list from gh).
	// The CI/CD tile shows "no run yet" rather than treating this as
	// a hard failure.
	ErrRunNotFound = errors.New("github: workflow run not found")

	// ErrStepsParseError is returned when GitHub responds with a
	// payload that does not include the expected `jobs[].steps[]`
	// shape. Usually means a `gh` version skew or an org-wide API
	// schema change worth investigating.
	ErrStepsParseError = errors.New("github: workflow steps parse error")
)
