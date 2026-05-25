package bento

import (
	"strings"
	"testing"
)

func TestParseTileBlock_DegenerateTwoLine(t *testing.T) {
	t.Parallel()

	got := parseTileBlock("first\nsecond")
	if got.TopBorder != "" || got.BottomBorder != "" {
		t.Errorf("expected empty borders for degenerate input, got top=%q bottom=%q",
			got.TopBorder, got.BottomBorder)
	}
	if len(got.Body) != 2 {
		t.Errorf("expected both lines in Body, got %d lines", len(got.Body))
	}
}

func TestParseTileBlock_BorderedRoundTrip(t *testing.T) {
	t.Parallel()

	in := strings.Join([]string{
		"┏━━━┓",
		"┃ X ┃",
		"┃ a ┃",
		"┃ b ┃",
		"┗━━━┛",
	}, "\n")

	got := parseTileBlock(in)

	if got.TopBorder != "┏━━━┓" {
		t.Errorf("TopBorder = %q", got.TopBorder)
	}
	if got.BottomBorder != "┗━━━┛" {
		t.Errorf("BottomBorder = %q", got.BottomBorder)
	}
	if len(got.Header) != 1 || got.Header[0] != "┃ X ┃" {
		t.Errorf("Header = %v", got.Header)
	}
	if len(got.Body) != 2 || got.Body[0] != "┃ a ┃" || got.Body[1] != "┃ b ┃" {
		t.Errorf("Body = %v", got.Body)
	}

	if got.Render() != in {
		t.Errorf("Render round-trip failed:\n--- got ---\n%s\n--- want ---\n%s", got.Render(), in)
	}
}

func TestClipBlock_NoopWhenWithinBudget(t *testing.T) {
	t.Parallel()

	block := TileBlock{
		TopBorder:    "┏━┓",
		Header:       []string{"┃H┃"},
		Body:         []string{"┃a┃"},
		BottomBorder: "┗━┛",
	}
	if got := clipBlock(block, 100); got.LineCount() != 4 {
		t.Errorf("clipBlock altered fitting block, got %d lines", got.LineCount())
	}
}

func TestClipBlock_ClipsBodyAndAppendsIndicator(t *testing.T) {
	t.Parallel()

	const tileInner = 60 // wide enough that the indicator text fits on one row
	body := make([]string, 10)
	for i := range body {
		body[i] = "┃ " + strings.Repeat("x", tileInner-4) + " ┃"
	}
	block := TileBlock{
		TopBorder:    "┏" + strings.Repeat("━", tileInner-2) + "┓",
		Header:       []string{"┃ HDR" + strings.Repeat(" ", tileInner-7) + "┃"},
		Body:         body,
		BottomBorder: "┗" + strings.Repeat("━", tileInner-2) + "┛",
	}
	got := clipBlock(block, 6)
	if got.LineCount() != 6 {
		t.Errorf("expected 6 lines after clip, got %d:\n%s", got.LineCount(), got.Render())
	}
	if !strings.Contains(got.Render(), "more lines") {
		t.Errorf("expected indicator row, got:\n%s", got.Render())
	}
	if got.TopBorder != block.TopBorder || got.BottomBorder != block.BottomBorder {
		t.Errorf("borders not preserved: top=%q bottom=%q", got.TopBorder, got.BottomBorder)
	}
}

func TestClipBlock_DegenerateBlockSlicedNaively(t *testing.T) {
	t.Parallel()

	block := TileBlock{
		Body: []string{"a", "b", "c", "d"},
	}
	got := clipBlock(block, 2)
	if got.LineCount() != 2 {
		t.Errorf("expected naïve slice to 2 lines, got %d", got.LineCount())
	}
	if got.TopBorder != "" || got.BottomBorder != "" {
		t.Errorf("naïve slice should not synthesize borders, got top=%q bottom=%q",
			got.TopBorder, got.BottomBorder)
	}
}

// TestClipTileBlock_LegacyEntryPointStillWorks pins the public
// behaviour of the legacy string-level wrapper: the Sprint 14
// refactor moved the arithmetic into [clipBlock] but every existing
// caller (engine, tests) goes through `clipTileBlock`. A regression
// here would surface as a broken cockpit silhouette in any
// terminal narrower than the row budget.
func TestClipTileBlock_LegacyEntryPointStillWorks(t *testing.T) {
	t.Parallel()

	pad := func(s string) string {
		const width = 60
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}
	in := strings.Join([]string{
		"┏" + strings.Repeat("━", 58) + "┓",
		"┃" + pad(" HEADER") + "┃",
		"┃" + pad(" aaa") + "┃",
		"┃" + pad(" bbb") + "┃",
		"┃" + pad(" ccc") + "┃",
		"┃" + pad(" ddd") + "┃",
		"┃" + pad(" eee") + "┃",
		"┗" + strings.Repeat("━", 58) + "┛",
	}, "\n")
	got := clipTileBlock(in, 5)
	if strings.Count(got, "\n")+1 > 5 {
		t.Errorf("clipTileBlock did not clip to 5 lines:\n%s", got)
	}
	if !strings.Contains(got, "more lines") {
		t.Errorf("clipTileBlock did not inject indicator:\n%s", got)
	}
}
