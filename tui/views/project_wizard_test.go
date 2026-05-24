package views

import (
	"os"
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
)

func TestProjectWizardGoldenViews(t *testing.T) {
	t.Parallel()

	screen := goldenScreen()
	screen.ProjectForm = ProjectWizardSnapshot{
		Step:         projectStepReview,
		ProfileAlias: "main",
		Stack:        "vite-react",
		Domain:       "app.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	assertGoldenNeedles(t, RenderProjectWizard(withSize(screen, 80, 24)), "testdata/project_wizard_80x24.golden.txt")
	assertGoldenNeedles(t, RenderProjectWizard(withSize(screen, 100, 30)), "testdata/project_wizard_100x30.golden.txt")
}

func TestInitWizardGoldenViews(t *testing.T) {
	t.Parallel()

	screen := goldenScreen()
	screen.InitForm = InitWizardSnapshot{
		Step:  5,
		Alias: "main",
		Host:  "s1.small.pl",
		Port:  "22",
		User:  "demo",
	}
	assertGoldenNeedles(t, RenderInitWizard(withSize(screen, 80, 24)), "testdata/init_wizard_80x24.golden.txt")
	assertGoldenNeedles(t, RenderInitWizard(withSize(screen, 100, 30)), "testdata/init_wizard_100x30.golden.txt")
}

func goldenScreen() Screen {
	return Screen{
		Styles:  theme.NewStyles(theme.Default()),
		Spinner: "dots",
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
		},
	}
}

func withSize(screen Screen, width, height int) Screen {
	screen.Width = width
	screen.Height = height
	return screen
}

func assertGoldenNeedles(t *testing.T, rendered, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(rendered, line) {
			t.Fatalf("rendered view missing golden line %q\n--- rendered ---\n%s", line, rendered)
		}
	}
}
