package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LiangNing7/agy-usage/internal/model"
)

func TestParseWorkspaceURIList(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		home     string
		wantProj string
	}{
		{
			name:     "unix project uri",
			raw:      `["file:///home/alice/workspace/nora"]`,
			home:     "/home/alice",
			wantProj: "nora",
		},
		{
			name:     "unix home dir",
			raw:      `["file:///home/alice"]`,
			home:     "/home/alice",
			wantProj: "~ (home)",
		},
		{
			name:     "windows encoded uri",
			raw:      `["file:///c%3A/Users/Bob/Repos/my-app"]`,
			home:     `C:\Users\Bob`,
			wantProj: "my-app",
		},
		{
			name:     "windows home dir",
			raw:      `["file:///C:/Users/Bob/"]`,
			home:     `C:\Users\Bob`,
			wantProj: "~ (home)",
		},
		{
			name:     "empty array",
			raw:      `[]`,
			home:     "/home/alice",
			wantProj: "(no-workspace)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, gotProj := ParseWorkspaceURIList(tc.raw, tc.home)
			if gotProj != tc.wantProj {
				t.Errorf("got project %q, want %q", gotProj, tc.wantProj)
			}
		})
	}
}

func TestCleanUserPromptTitle(t *testing.T) {
	raw := "<USER_REQUEST>\n  统计一下到现在，我一共用了多少token了？  \n第二行内容\n</USER_REQUEST>\n<ADDITIONAL_METADATA>..."
	got := CleanUserPromptTitle(raw)
	if got != "统计一下到现在，我一共用了多少token了？" {
		t.Fatalf("unexpected title: %q", got)
	}
}

func TestCollectLargeLinesAndCheckpoints(t *testing.T) {
	tmp := t.TempDir()
	cliDir := filepath.Join(tmp, "cli")
	logDir := filepath.Join(cliDir, "brain", "conv-12345678", ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Include a >100KB line to verify no 64KB scanner truncation occurs.
	hugePadding := strings.Repeat("A", 120*1024)
	jsonl := strings.Join([]string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-11-25T01:00:00Z","content":"<USER_REQUEST>\nBuild token board\n</USER_REQUEST>"}`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-11-25T01:00:05Z","model":"gemini-4-carbon-high","input_tokens":1000,"cache_read_tokens":4000,"output_tokens":250,"content":"` + hugePadding + `"}`,
		`{"step_index":2,"source":"SYSTEM","type":"CHECKPOINT","created_at":"2026-11-25T01:05:00Z","input_tokens":300,"cache_read_tokens":5000,"output_tokens":500}`,
		`{corrupted_half_written_json_line`,
	}, "\n")

	if err := os.WriteFile(filepath.Join(logDir, "transcript.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Collect(filepath.Join(tmp, "ide"), cliDir, time.UTC)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 token steps, got %d", len(res.Steps))
	}
	cs := res.Convs["conv-12345678"]
	if cs == nil {
		t.Fatalf("missing conv stats")
	}
	if cs.Surface != model.SurfaceCLI {
		t.Errorf("expected CLI surface, got %s", cs.Surface)
	}
	if cs.Title != "Build token board" {
		t.Errorf("expected extracted title 'Build token board', got %q", cs.Title)
	}
	if cs.Compactions != 1 {
		t.Errorf("expected 1 compaction, got %d", cs.Compactions)
	}
	if cs.TotalTokens() != (1000+4000+250)+(300+5000+500) {
		t.Errorf("unexpected total tokens: %d", cs.TotalTokens())
	}
}
