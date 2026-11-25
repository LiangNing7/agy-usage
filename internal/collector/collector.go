package collector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/LiangNing7/agy-usage/internal/model"
	"github.com/LiangNing7/agy-usage/internal/sqlite"
)

var (
	userReqRE    = regexp.MustCompile(`(?s)<USER_REQUEST>\s*(.*?)\s*</USER_REQUEST>`)
	multiSpaceRE = regexp.MustCompile(`\s+`)
)

// DefaultPaths returns the default Antigravity IDE/App and CLI data directories
// across Linux, macOS, and Windows, respecting AGY_DATA_DIR and AGY_CLI_DATA_DIR.
func DefaultPaths() (agyDir string, cliDir string) {
	home, _ := os.UserHomeDir()
	agyDir = os.Getenv("AGY_DATA_DIR")
	if agyDir == "" && home != "" {
		agyDir = filepath.Join(home, ".gemini", "antigravity")
	}
	cliDir = os.Getenv("AGY_CLI_DATA_DIR")
	if cliDir == "" && home != "" {
		cliDir = filepath.Join(home, ".gemini", "antigravity-cli")
	}
	return agyDir, cliDir
}

// Result holds collected steps, per-conversation stats, and any summary-only conversations
// whose transcript logs were not readable (e.g., when executed inside a restricted sandbox).
type Result struct {
	Steps        []model.StepUsage
	Convs        map[string]*model.ConvStats
	Inaccessible []model.ConvSummaryMeta
}

type transcriptStep struct {
	StepIndex       int    `json:"step_index"`
	Source          string `json:"source"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	Content         string `json:"content"`
	Model           string `json:"model"`
	InputTokens     *int64 `json:"input_tokens"`
	CacheReadTokens *int64 `json:"cache_read_tokens"`
	OutputTokens    *int64 `json:"output_tokens"`
}

// Collect scans Antigravity IDE/App and CLI data directories and returns parsed token usage records.
func Collect(agyDir, cliDir string, loc *time.Location) (*Result, error) {
	if loc == nil {
		loc = time.Local
	}
	metas := loadSummaryDB(filepath.Join(agyDir, "conversation_summaries.db"), loc)
	// Also check if CLI has its own conversation_summaries.db (future-proofing).
	if cliDir != "" && cliDir != agyDir {
		cliMetas := loadSummaryDB(filepath.Join(cliDir, "conversation_summaries.db"), loc)
		for k, v := range cliMetas {
			if _, exists := metas[k]; !exists {
				metas[k] = v
			}
		}
	}

	type discoveredLog struct {
		path    string
		surface model.Surface
	}
	transcripts := make(map[string]discoveredLog)

	discoverSurface := func(baseDir string, surf model.Surface) {
		if baseDir == "" {
			return
		}
		brainDir := filepath.Join(baseDir, "brain")
		entries, err := os.ReadDir(brainDir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			cid := ent.Name()
			tp := filepath.Join(brainDir, cid, ".system_generated", "logs", "transcript.jsonl")
			if info, err := os.Stat(tp); err == nil && !info.IsDir() {
				transcripts[cid] = discoveredLog{path: tp, surface: surf}
			}
		}
	}

	discoverSurface(agyDir, model.SurfaceIDE)
	discoverSurface(cliDir, model.SurfaceCLI)

	cids := make([]string, 0, len(transcripts))
	for cid := range transcripts {
		cids = append(cids, cid)
	}
	sort.Strings(cids)

	res := &Result{
		Convs: make(map[string]*model.ConvStats, len(cids)),
	}

	for _, cid := range cids {
		dl := transcripts[cid]
		meta, hasMeta := metas[cid]

		surface := dl.surface
		project := "(unknown)"
		workspace := ""
		parentID := ""
		nestingDepth := 0
		title := ""

		if hasMeta {
			surface = meta.Surface
			project = meta.Project
			workspace = meta.Workspace
			parentID = meta.ParentID
			nestingDepth = meta.NestingDepth
			title = meta.Title
		}

		cs := &model.ConvStats{
			ConvID:       cid,
			Title:        title,
			Surface:      surface,
			Project:      project,
			Workspace:    workspace,
			ParentID:     parentID,
			NestingDepth: nestingDepth,
			Models:       make(map[string]struct{}),
		}

		mtime := time.Now().In(loc)
		if st, err := os.Stat(dl.path); err == nil {
			mtime = st.ModTime().In(loc)
		}

		fallbackTitle := ""
		if err := scanTranscriptFile(dl.path, func(ts transcriptStep) {
			if cs.Title == "" && fallbackTitle == "" && ts.Type == "USER_INPUT" && ts.Content != "" {
				fallbackTitle = CleanUserPromptTitle(ts.Content)
			}

			parsedTS, hasTS := ParseTimestamp(ts.CreatedAt, loc)
			if hasTS {
				if cs.FirstTS.IsZero() || parsedTS.Before(cs.FirstTS) {
					cs.FirstTS = parsedTS
				}
				if cs.LastTS.IsZero() || parsedTS.After(cs.LastTS) {
					cs.LastTS = parsedTS
				}
			}

			if ts.InputTokens == nil && ts.CacheReadTokens == nil && ts.OutputTokens == nil {
				return
			}

			var uIn, cRd, out int64
			if ts.InputTokens != nil {
				uIn = *ts.InputTokens
			}
			if ts.CacheReadTokens != nil {
				cRd = *ts.CacheReadTokens
			}
			if ts.OutputTokens != nil {
				out = *ts.OutputTokens
			}

			modelName := strings.TrimSpace(ts.Model)
			if modelName != "" {
				cs.Models[modelName] = struct{}{}
			} else if ts.Type == "CHECKPOINT" {
				modelName = "[context-compaction]"
			} else {
				modelName = "[unknown]"
			}

			stepTime := parsedTS
			if !hasTS {
				if !cs.LastTS.IsZero() {
					stepTime = cs.LastTS
				} else {
					stepTime = mtime
				}
			}

			su := model.StepUsage{
				ConvID:        cid,
				Surface:       surface,
				Project:       project,
				IsSubagent:    parentID != "" || nestingDepth > 0,
				Timestamp:     stepTime,
				LocalDate:     stepTime.Format("2006-01-02"),
				Model:         modelName,
				StepType:      ts.Type,
				UncachedInput: uIn,
				CacheRead:     cRd,
				Output:        out,
			}
			res.Steps = append(res.Steps, su)

			cs.Steps++
			if ts.Type == "CHECKPOINT" {
				cs.Compactions++
			}
			cs.UncachedInput += uIn
			cs.CacheRead += cRd
			cs.Output += out
		}); err != nil {
			continue
		}

		if cs.Title == "" {
			prefix := cid
			if len(prefix) > 8 {
				prefix = prefix[:8]
			}
			switch {
			case fallbackTitle != "":
				cs.Title = fallbackTitle
			case parentID != "":
				cs.Title = "(Subagent " + prefix + ")"
			default:
				cs.Title = "(Untitled " + prefix + ")"
			}
		}
		if cs.LastTS.IsZero() {
			if hasMeta && !meta.LastModified.IsZero() {
				cs.LastTS = meta.LastModified
			} else {
				cs.LastTS = mtime
			}
		}
		res.Convs[cid] = cs
	}

	for cid, m := range metas {
		if _, ok := transcripts[cid]; !ok && m.SummarySteps > 0 {
			res.Inaccessible = append(res.Inaccessible, m)
		}
	}
	sort.Slice(res.Inaccessible, func(i, j int) bool {
		return res.Inaccessible[i].ConvID < res.Inaccessible[j].ConvID
	})

	return res, nil
}

// scanTranscriptFile streams JSONL lines of arbitrary length without bufio.Scanner's 64KB cap.
func scanTranscriptFile(path string, fn func(transcriptStep)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 128*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				var step transcriptStep
				if jerr := json.Unmarshal(trimmed, &step); jerr == nil {
					fn(step)
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func loadSummaryDB(dbPath string, loc *time.Location) map[string]model.ConvSummaryMeta {
	out := make(map[string]model.ConvSummaryMeta)
	if dbPath == "" {
		return out
	}
	if _, err := os.Stat(dbPath); err != nil {
		return out
	}
	rows, err := sqlite.ReadTable(dbPath, "conversation_summaries")
	if err != nil {
		return out
	}
	home, _ := os.UserHomeDir()
	for _, r := range rows {
		cid := strings.TrimSpace(r.String("conversation_id"))
		if cid == "" {
			continue
		}
		wsPath, proj := ParseWorkspaceURIList(r.String("workspace_uris"), home)
		src := strings.ToUpper(r.String("source"))
		surf := model.SurfaceIDE
		if strings.Contains(src, "CLI") {
			surf = model.SurfaceCLI
		}
		lm, _ := ParseTimestamp(r.String("last_modified_time"), loc)
		out[cid] = model.ConvSummaryMeta{
			ConvID:       cid,
			Title:        strings.TrimSpace(r.String("title")),
			Surface:      surf,
			Workspace:    wsPath,
			Project:      proj,
			ParentID:     strings.TrimSpace(r.String("parent_conversation_id")),
			NestingDepth: int(r.Int64("nesting_depth")),
			SummarySteps: int(r.Int64("step_count")),
			LastModified: lm,
		}
	}
	return out
}

// ParseWorkspaceURIList parses Antigravity's JSON array of file:// URIs across Unix and Windows.
func ParseWorkspaceURIList(rawJSON string, homeDir string) (workspace string, project string) {
	if strings.TrimSpace(rawJSON) == "" {
		return "", "(no-workspace)"
	}
	var uris []string
	if err := json.Unmarshal([]byte(rawJSON), &uris); err != nil || len(uris) == 0 {
		return "", "(no-workspace)"
	}
	u := strings.TrimSpace(uris[0])
	if strings.HasPrefix(strings.ToLower(u), "file://") {
		if parsed, err := url.Parse(u); err == nil {
			p := parsed.Path
			if decoded, derr := url.PathUnescape(p); derr == nil {
				p = decoded
			}
			// Handle Windows file:///C:/Users/... paths
			if len(p) >= 3 && p[0] == '/' && ((p[1] >= 'A' && p[1] <= 'Z') || (p[1] >= 'a' && p[1] <= 'z')) && p[2] == ':' {
				p = p[1:]
			}
			if parsed.Host != "" && parsed.Host != "localhost" {
				p = "//" + parsed.Host + p
			}
			u = p
		} else {
			u = strings.TrimPrefix(u, "file://")
		}
	}

	// Normalize trailing slashes while preserving root "/" or "C:"
	norm := strings.ReplaceAll(u, "\\", "/")
	for len(norm) > 1 && strings.HasSuffix(norm, "/") && !strings.HasSuffix(norm, ":/") {
		norm = strings.TrimSuffix(norm, "/")
	}
	if norm == "" {
		return "", "(no-workspace)"
	}

	normHome := strings.TrimRight(strings.ReplaceAll(homeDir, "\\", "/"), "/")
	if normHome != "" && strings.EqualFold(norm, normHome) {
		return u, "~ (home)"
	}

	trimmed := strings.TrimRight(norm, "/")
	if idx := strings.LastIndexByte(trimmed, '/'); idx >= 0 && idx+1 < len(trimmed) {
		return u, trimmed[idx+1:]
	}
	return u, norm
}

// CleanUserPromptTitle extracts a concise readable title from the first USER_INPUT step.
func CleanUserPromptTitle(content string) string {
	body := content
	if m := userReqRE.FindStringSubmatch(content); len(m) > 1 {
		body = m[1]
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "<") {
			continue
		}
		line = multiSpaceRE.ReplaceAllString(line, " ")
		runes := []rune(line)
		if len(runes) > 80 {
			return string(runes[:80])
		}
		return line
	}
	return ""
}

// ParseTimestamp parses ISO8601 / SQLite datetime strings into the target local timezone.
func ParseTimestamp(raw string, loc *time.Location) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	if strings.Contains(s, " ") && !strings.Contains(s, "T") {
		s = strings.Replace(s, " ", "T", 1)
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.In(loc), true
		}
	}
	return time.Time{}, false
}
