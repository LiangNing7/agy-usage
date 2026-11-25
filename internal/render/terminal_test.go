package render

import (
	"strings"
	"testing"
	"time"

	"github.com/LiangNing7/agy-usage/internal/collector"
	"github.com/LiangNing7/agy-usage/internal/model"
	"github.com/LiangNing7/agy-usage/internal/stats"
)

func TestDashboardAlignmentAcrossWidthsAndLanguages(t *testing.T) {
	now := time.Date(2026, 11, 25, 16, 30, 0, 0, time.UTC)
	raw := &collector.Result{
		Steps: []model.StepUsage{
			{
				ConvID: "root-conv-001", Surface: model.SurfaceIDE, Project: "nora",
				IsSubagent: false, Timestamp: now.Add(-24 * time.Hour), LocalDate: "2026-11-24",
				Model: "gemini-4-neon-high", StepType: "PLANNER_RESPONSE",
				UncachedInput: 5_000_000, CacheRead: 25_000_000, Output: 400_000,
			},
			{
				ConvID: "sub-conv-002", Surface: model.SurfaceIDE, Project: "(no-workspace)",
				IsSubagent: true, Timestamp: now, LocalDate: "2026-11-25",
				Model: "gemini-4-carbon-medium", StepType: "PLANNER_RESPONSE",
				UncachedInput: 1_200_000, CacheRead: 8_000_000, Output: 120_000,
			},
			{
				ConvID: "cli-conv-003", Surface: model.SurfaceCLI, Project: "~ (home)",
				IsSubagent: false, Timestamp: now, LocalDate: "2026-11-25",
				Model: "[context-compaction]", StepType: "CHECKPOINT",
				UncachedInput: 50_000, CacheRead: 300_000, Output: 8_000,
			},
		},
		Convs: map[string]*model.ConvStats{
			"root-conv-001": {
				ConvID: "root-conv-001", Title: "Nora 功能与架构演进设计探索",
				Surface: model.SurfaceIDE, Project: "nora", Models: map[string]struct{}{},
			},
			"sub-conv-002": {
				ConvID: "sub-conv-002", Title: "(Subagent sub-conv)",
				Surface: model.SurfaceIDE, Project: "(no-workspace)", ParentID: "root-conv-001", NestingDepth: 1,
				Models: map[string]struct{}{},
			},
			"cli-conv-003": {
				ConvID: "cli-conv-003", Title: "统计一下到现在，我一共用了多少token了？",
				Surface: model.SurfaceCLI, Project: "~ (home)", Models: map[string]struct{}{},
			},
		},
		Inaccessible: []model.ConvSummaryMeta{
			{ConvID: "hidden-1", SummarySteps: 42},
		},
	}

	for _, flat := range []bool{false, true} {
		rep := stats.Aggregate(raw, model.FilterOptions{
			Surface: model.SurfaceAll,
			Flat:    flat,
			SortBy:  "tokens",
			TopN:    10,
		}, now)

		if !flat && len(rep.Sessions) != 2 {
			t.Fatalf("expected 2 rolled-up root sessions, got %d", len(rep.Sessions))
		}
		if flat && len(rep.Sessions) != 3 {
			t.Fatalf("expected 3 flat sessions, got %d", len(rep.Sessions))
		}

		for _, lang := range []Lang{LangZH, LangEN} {
			for _, rawFmt := range []bool{false, true} {
				for _, cols := range []int{86, 96, 112, 128} {
					out := RenderTerminal(rep, Options{
						Color: true,
						Raw:   rawFmt,
						Flat:  flat,
						TopN:  10,
						Sort:  "tokens",
						Width: cols,
						Lang:  lang,
					})
					lines := strings.Split(out, "\n")
					for idx, line := range lines {
						gotW := DispWidth(line)
						if gotW != cols {
							t.Fatalf("width mismatch (lang=%s flat=%v raw=%v cols=%d) line %d: got %d, want %d\nline: %s",
								lang, flat, rawFmt, cols, idx, gotW, cols, StripANSI(line))
						}
					}
				}
			}
		}
	}
}
