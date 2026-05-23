package github

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCLITransport_RedactsPATFromErrors(t *testing.T) {
	t.Parallel()

	token := "gh" + "p_" + "123456789012345678901234567890123456"
	runner := commandRunnerFunc(func(context.Context, string, []string, []byte) ([]byte, []byte, error) {
		return nil, []byte("remote: " + token + " leaked"), errors.New("exit status 1")
	})
	transport := NewCLITransport(runner)

	_, err := transport.CreateRepo(context.Background(), CreateRepoRequest{
		Name:       "demo",
		Visibility: VisibilityPrivate,
	})
	if err == nil {
		t.Fatal("CreateRepo err = nil, want redacted failure")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked PAT: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %v, want redacted marker", err)
	}
}

func TestCLITransport_SetActionsSecretDoesNotPassSecretInArgs(t *testing.T) {
	t.Parallel()

	secret := "gh" + "s_" + "123456789012345678901234567890123456"
	var gotArgs []string
	var gotStdin []byte
	runner := commandRunnerFunc(func(_ context.Context, _ string, args []string, stdin []byte) ([]byte, []byte, error) {
		gotArgs = append([]string(nil), args...)
		gotStdin = append([]byte(nil), stdin...)
		return []byte("{}"), nil, nil
	})
	transport := NewCLITransport(runner)

	err := transport.SetActionsSecret(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, "SSH_PRIVATE_KEY", []byte(secret))
	if err != nil {
		t.Fatalf("SetActionsSecret: %v", err)
	}
	if strings.Contains(strings.Join(gotArgs, " "), secret) {
		t.Fatalf("secret appeared in gh args: %q", gotArgs)
	}
	if !bytes.Equal(gotStdin, []byte(secret)) {
		t.Fatalf("stdin = %q, want secret bytes", gotStdin)
	}
}

func TestRESTTransport_MapsRepositoryExistsAndRedactsPAT(t *testing.T) {
	t.Parallel()

	token := "github_" + "pat_" + "12345678901234567890123456789012345678901234567890123456789012345678901234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization header = %q", got)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"repository already exists for ` + token + `"}`))
	}))
	t.Cleanup(server.Close)

	transport := NewRESTTransport(RESTOptions{
		BaseURL: server.URL,
		TokenSource: tokenSourceFunc(func(context.Context) (string, error) {
			return token, nil
		}),
	})

	_, err := transport.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if !errors.Is(err, ErrRepoExists) {
		t.Fatalf("CreateRepo err = %v, want ErrRepoExists", err)
	}
	if strings.Contains(err.Error(), "github_pat_") {
		t.Fatalf("error leaked PAT: %v", err)
	}
}

func TestEncryptSecretForGitHubSealedBoxDoesNotExposePlaintext(t *testing.T) {
	t.Parallel()

	publicKey := "11qYAYKxCrfVSjGEx1Q+Am1F0ndS9uDsLq5AT9DGIaA="
	got, err := EncryptSecretForGitHub(publicKey, []byte("super-secret-value"))
	if err != nil {
		t.Fatalf("EncryptSecretForGitHub: %v", err)
	}
	if strings.Contains(got, "super-secret-value") {
		t.Fatalf("encrypted value contains plaintext: %q", got)
	}
	if got == "" {
		t.Fatal("encrypted value is empty")
	}
}

func TestClientFallsBackToRESTWhenGHUnavailable(t *testing.T) {
	t.Parallel()

	primary := transportFunc{
		createRepo: func(context.Context, CreateRepoRequest) (*Repository, error) {
			return nil, ErrGHUnavailable
		},
	}
	fallback := transportFunc{
		createRepo: func(context.Context, CreateRepoRequest) (*Repository, error) {
			return &Repository{Name: "demo", Owner: "dilitS"}, nil
		},
	}
	client := NewClient(Options{Primary: primary, Fallback: fallback})

	repo, err := client.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if repo.FullName() != "dilitS/demo" {
		t.Fatalf("FullName = %q, want dilitS/demo", repo.FullName())
	}
}

type commandRunnerFunc func(context.Context, string, []string, []byte) ([]byte, []byte, error)

func (fn commandRunnerFunc) Run(ctx context.Context, name string, args []string, stdin []byte) (stdout, stderr []byte, err error) {
	return fn(ctx, name, args, stdin)
}

type tokenSourceFunc func(context.Context) (string, error)

func (fn tokenSourceFunc) Token(ctx context.Context) (string, error) { return fn(ctx) }

type transportFunc struct {
	createRepo       func(context.Context, CreateRepoRequest) (*Repository, error)
	addDeployKey     func(context.Context, RepoRef, DeployKeyRequest) (*DeployKey, error)
	setActionsSecret func(context.Context, RepoRef, string, []byte) error
	dispatchWorkflow func(context.Context, RepoRef, DispatchWorkflowRequest) (*WorkflowDispatch, error)
	getLatestRun     func(context.Context, RepoRef, LatestRunRequest) (*WorkflowRun, error)
}

func (t transportFunc) CreateRepo(ctx context.Context, req CreateRepoRequest) (*Repository, error) {
	return t.createRepo(ctx, req)
}

func (t transportFunc) AddDeployKey(ctx context.Context, repo RepoRef, req DeployKeyRequest) (*DeployKey, error) {
	return t.addDeployKey(ctx, repo, req)
}

func (t transportFunc) SetActionsSecret(ctx context.Context, repo RepoRef, name string, value []byte) error {
	return t.setActionsSecret(ctx, repo, name, value)
}

func (t transportFunc) DispatchWorkflow(ctx context.Context, repo RepoRef, req DispatchWorkflowRequest) (*WorkflowDispatch, error) {
	return t.dispatchWorkflow(ctx, repo, req)
}

func (t transportFunc) GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error) {
	return t.getLatestRun(ctx, repo, req)
}
