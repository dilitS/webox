package wizard_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/wizard"
)

func TestExecuteGitHubProvisionSuccessPushesCleanupInOrder(t *testing.T) {
	t.Parallel()

	gh := &fakeGitHubProvisioner{}
	stack := wizard.NewStack(nil, "wizard-1")
	report, err := wizard.ExecuteGitHubProvision(context.Background(), gh, sampleGitHubPlan(), stack)
	if err != nil {
		t.Fatalf("ExecuteGitHubProvision: %v", err)
	}
	if report == nil || report.Workflow == nil || report.Dispatch == nil {
		t.Fatalf("report missing workflow/dispatch: %+v", report)
	}
	steps := stack.Steps()
	kinds := make([]string, 0, len(steps))
	for _, step := range steps {
		kinds = append(kinds, string(step.Kind))
	}
	want := "github_repo,github_deploy_key,github_actions_secret,github_actions_secret,github_workflow_file"
	if strings.Join(kinds, ",") != want {
		t.Fatalf("cleanup kinds = %s, want %s", strings.Join(kinds, ","), want)
	}
}

func TestExecuteGitHubProvisionDispatchFailureKeepsRollbackStack(t *testing.T) {
	t.Parallel()

	gh := &fakeGitHubProvisioner{dispatchErr: github.ErrWorkflowDispatchFailed}
	stack := wizard.NewStack(nil, "wizard-1")
	_, err := wizard.ExecuteGitHubProvision(context.Background(), gh, sampleGitHubPlan(), stack)
	if !errors.Is(err, github.ErrWorkflowDispatchFailed) {
		t.Fatalf("err = %v, want ErrWorkflowDispatchFailed", err)
	}
	if stack.Len() != 5 {
		t.Fatalf("rollback stack len = %d, want 5", stack.Len())
	}
}

func sampleGitHubPlan() wizard.GitHubProvisionPlan {
	return wizard.GitHubProvisionPlan{
		Owner:           "dilitS",
		Repo:            "demo",
		Branch:          "main",
		Visibility:      github.VisibilityPrivate,
		DeployPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDemoKey webox",
		Secrets: map[string][]byte{
			"DEPLOY_HOST": []byte("s1.small.pl"),
			"DEPLOY_USER": []byte("demo"),
		},
		WorkflowPath:    ".github/workflows/deploy.yml",
		WorkflowContent: []byte("name: webox deploy\n"),
	}
}

type fakeGitHubProvisioner struct {
	dispatchErr error
}

func (f *fakeGitHubProvisioner) CreateRepo(context.Context, github.CreateRepoRequest) (*github.Repository, error) {
	return &github.Repository{Owner: "dilitS", Name: "demo", HTMLURL: "https://github.com/dilitS/demo"}, nil
}

func (f *fakeGitHubProvisioner) AddDeployKey(context.Context, github.RepoRef, github.DeployKeyRequest) (*github.DeployKey, error) {
	return &github.DeployKey{ID: 42, Title: "webox"}, nil
}

func (f *fakeGitHubProvisioner) SetActionsSecret(context.Context, github.RepoRef, string, []byte) error {
	return nil
}

func (f *fakeGitHubProvisioner) CommitWorkflowFile(context.Context, github.RepoRef, github.CommitFileRequest) error {
	return nil
}

func (f *fakeGitHubProvisioner) DispatchWorkflow(context.Context, github.RepoRef, github.DispatchWorkflowRequest) (*github.WorkflowDispatch, error) {
	if f.dispatchErr != nil {
		return nil, f.dispatchErr
	}
	return &github.WorkflowDispatch{RunID: 7, HTMLURL: "https://github.com/dilitS/demo/actions/runs/7"}, nil
}
