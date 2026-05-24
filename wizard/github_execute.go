package wizard

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/dilitS/webox/services/github"
)

// GitHubProvisionPlan is the secret-free plan for GitHub-side setup.
type GitHubProvisionPlan struct {
	Owner           string
	Repo            string
	Branch          string
	Visibility      github.Visibility
	DeployPublicKey string
	Secrets         map[string][]byte
	WorkflowPath    string
	WorkflowContent []byte
}

// GitHubProvisionReport captures successful GitHub setup milestones.
type GitHubProvisionReport struct {
	Repository *github.Repository
	DeployKey  *github.DeployKey
	Workflow   *github.CommitFileRequest
	Dispatch   *github.WorkflowDispatch
}

// GitHubProvisioner is the wizard seam implemented by services/github.
type GitHubProvisioner interface {
	CreateRepo(ctx context.Context, req github.CreateRepoRequest) (*github.Repository, error)
	AddDeployKey(ctx context.Context, repo github.RepoRef, req github.DeployKeyRequest) (*github.DeployKey, error)
	SetActionsSecret(ctx context.Context, repo github.RepoRef, name string, value []byte) error
	CommitWorkflowFile(ctx context.Context, repo github.RepoRef, req github.CommitFileRequest) error
	DispatchWorkflow(ctx context.Context, repo github.RepoRef, req github.DispatchWorkflowRequest) (*github.WorkflowDispatch, error)
}

// ExecuteGitHubProvision creates GitHub resources and pushes cleanup steps
// after each successful external mutation.
func ExecuteGitHubProvision(ctx context.Context, gh GitHubProvisioner, plan GitHubProvisionPlan, stack *Stack) (*GitHubProvisionReport, error) {
	if gh == nil {
		return nil, fmt.Errorf("%w: github provisioner is nil", ErrInvalidPlan)
	}
	if stack == nil {
		return nil, fmt.Errorf("%w: stack is nil", ErrInvalidPlan)
	}
	if err := validateGitHubProvisionPlan(plan); err != nil {
		return nil, err
	}

	report := &GitHubProvisionReport{}
	repo, err := gh.CreateRepo(ctx, github.CreateRepoRequest{
		Owner:      plan.Owner,
		Name:       plan.Repo,
		Visibility: plan.Visibility,
		AutoInit:   true,
	})
	if err != nil {
		return report, err
	}
	report.Repository = repo
	ref := repo.Ref()
	if ref.Owner == "" {
		ref.Owner = plan.Owner
	}
	if ref.Name == "" {
		ref.Name = plan.Repo
	}
	if err := stack.Push(ctx, CleanupStep{
		Name:      "Delete GitHub repo " + ref.FullName(),
		Kind:      ResourceGitHubRepo,
		Params:    map[string]string{"owner": ref.Owner, "repo": ref.Name},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, err
	}

	key, err := gh.AddDeployKey(ctx, ref, github.DeployKeyRequest{
		Title:    "Webox deploy key",
		Key:      plan.DeployPublicKey,
		ReadOnly: false,
	})
	if err != nil {
		return report, err
	}
	report.DeployKey = key
	if err := stack.Push(ctx, CleanupStep{
		Name: "Remove GitHub deploy key " + strconv.FormatInt(key.ID, 10),
		Kind: ResourceGitHubDeployKey,
		Params: map[string]string{
			"owner": ref.Owner,
			"repo":  ref.Name,
			"keyID": strconv.FormatInt(key.ID, 10),
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, err
	}

	for _, name := range sortedSecretNames(plan.Secrets) {
		if err := gh.SetActionsSecret(ctx, ref, name, plan.Secrets[name]); err != nil {
			return report, err
		}
		if err := stack.Push(ctx, CleanupStep{
			Name:      "Remove GitHub Actions secret " + name,
			Kind:      ResourceGitHubActionsSecret,
			Params:    map[string]string{"owner": ref.Owner, "repo": ref.Name, "name": name},
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return report, err
		}
	}

	commitReq := github.CommitFileRequest{
		Path:    plan.WorkflowPath,
		Branch:  plan.Branch,
		Message: "Add Webox deploy workflow",
		Content: append([]byte(nil), plan.WorkflowContent...),
	}
	if err := gh.CommitWorkflowFile(ctx, ref, commitReq); err != nil {
		return report, err
	}
	report.Workflow = &commitReq
	if err := stack.Push(ctx, CleanupStep{
		Name: "Remove GitHub workflow " + plan.WorkflowPath,
		Kind: ResourceGitHubWorkflowFile,
		Params: map[string]string{
			"owner":  ref.Owner,
			"repo":   ref.Name,
			"path":   plan.WorkflowPath,
			"branch": plan.Branch,
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, err
	}

	dispatch, err := gh.DispatchWorkflow(ctx, ref, github.DispatchWorkflowRequest{
		WorkflowID: plan.WorkflowPath,
		Ref:        plan.Branch,
	})
	if err != nil {
		return report, err
	}
	report.Dispatch = dispatch
	return report, nil
}

func validateGitHubProvisionPlan(plan GitHubProvisionPlan) error {
	for name, value := range map[string]string{
		"owner":             plan.Owner,
		"repo":              plan.Repo,
		"branch":            plan.Branch,
		"deploy_public_key": plan.DeployPublicKey,
		"workflow_path":     plan.WorkflowPath,
	} {
		if value == "" {
			return fmt.Errorf("%w: github %s is required", ErrInvalidPlan, name)
		}
		if err := ValidateWorkflowField(name, value); err != nil {
			return err
		}
	}
	if len(plan.WorkflowContent) == 0 {
		return fmt.Errorf("%w: github workflow content is required", ErrInvalidPlan)
	}
	if len(plan.Secrets) == 0 {
		return fmt.Errorf("%w: at least one github action secret is required", ErrInvalidPlan)
	}
	return nil
}

func sortedSecretNames(secrets map[string][]byte) []string {
	names := make([]string, 0, len(secrets))
	for name := range secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
