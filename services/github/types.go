//nolint:revive // Public DTO comments would mostly restate field/type names; package docs define the contract.
package github

import (
	"context"
	"strings"
	"time"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

// RepoRef identifies a repository without carrying credentials.
type RepoRef struct {
	Owner string
	Name  string
}

func (r RepoRef) FullName() string {
	if r.Owner == "" {
		return r.Name
	}
	return r.Owner + "/" + r.Name
}

func (r RepoRef) validate() error {
	if strings.TrimSpace(r.Owner) == "" || strings.TrimSpace(r.Name) == "" {
		return ErrInvalidRepoRef
	}
	return nil
}

type CreateRepoRequest struct {
	Owner       string
	Name        string
	Description string
	Visibility  Visibility
	AutoInit    bool
}

type Repository struct {
	ID      int64  `json:"id"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	SSHURL  string `json:"ssh_url"`
}

func (r Repository) Ref() RepoRef {
	return RepoRef{Owner: r.Owner, Name: r.Name}
}

func (r Repository) FullName() string {
	return r.Ref().FullName()
}

type DeployKeyRequest struct {
	Title    string
	Key      string
	ReadOnly bool
}

type DeployKey struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Key   string `json:"key,omitempty"`
	URL   string `json:"url,omitempty"`
}

type DispatchWorkflowRequest struct {
	WorkflowID string
	Ref        string
	Inputs     map[string]string
}

type CommitFileRequest struct {
	Path    string
	Branch  string
	Message string
	Content []byte
}

type WorkflowDispatch struct {
	RunID   int64  `json:"workflow_run_id,omitempty"`
	RunURL  string `json:"run_url,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

type LatestRunRequest struct {
	Branch string
	Event  string
}

type WorkflowRun struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Conclusion string     `json:"conclusion"`
	HTMLURL    string     `json:"html_url"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	StartedAt  *time.Time `json:"run_started_at,omitempty"`
}

// Transport is implemented by gh CLI and REST fallback clients.
type Transport interface {
	CreateRepo(ctx context.Context, req CreateRepoRequest) (*Repository, error)
	AddDeployKey(ctx context.Context, repo RepoRef, req DeployKeyRequest) (*DeployKey, error)
	SetActionsSecret(ctx context.Context, repo RepoRef, name string, value []byte) error
	DispatchWorkflow(ctx context.Context, repo RepoRef, req DispatchWorkflowRequest) (*WorkflowDispatch, error)
	GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error)
}
