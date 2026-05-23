package github

import (
	"context"
	"strings"
	"testing"
)

func TestCLITransportCleanupUsesMetadataOnlyArgs(t *testing.T) {
	t.Parallel()

	var calls []string
	runner := commandRunnerFunc(func(_ context.Context, _ string, args []string, stdin []byte) ([]byte, []byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " --input -") && len(stdin) == 0 {
			t.Fatalf("cleanup command %q expected JSON stdin", joined)
		}
		calls = append(calls, joined)
		if strings.Contains(joined, "GET /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml") {
			return []byte(`{"sha":"abc123"}`), nil, nil
		}
		return []byte("{}"), nil, nil
	})
	transport := NewCLITransport(runner)

	if err := transport.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main"); err != nil {
		t.Fatalf("RemoveGitHubWorkflowFile: %v", err)
	}
	if err := transport.RemoveGitHubActionsSecret(context.Background(), "dilitS", "demo", "DEPLOY_HOST"); err != nil {
		t.Fatalf("RemoveGitHubActionsSecret: %v", err)
	}
	if err := transport.RemoveGitHubDeployKey(context.Background(), "dilitS", "demo", 42); err != nil {
		t.Fatalf("RemoveGitHubDeployKey: %v", err)
	}
	if err := transport.RemoveGitHubRepo(context.Background(), "dilitS", "demo"); err != nil {
		t.Fatalf("RemoveGitHubRepo: %v", err)
	}
	got := strings.Join(calls, "\n")
	for _, needle := range []string{
		"api --method GET /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml?ref=main",
		"api --method DELETE /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml",
		"api --method DELETE /repos/dilitS/demo/actions/secrets/DEPLOY_HOST",
		"api --method DELETE /repos/dilitS/demo/keys/42",
		"api --method DELETE /repos/dilitS/demo",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("cleanup calls missing %q:\n%s", needle, got)
		}
	}
}
