package stats

import (
	"sort"
	"strings"
	"time"

	"github.com/LiangNing7/agy-usage/internal/collector"
	"github.com/LiangNing7/agy-usage/internal/model"
)

// Aggregate applies user filters to collected data and builds a full Report.
func Aggregate(raw *collector.Result, opts model.FilterOptions, now time.Time) model.Report {
	loc := now.Location()
	todayStr := now.Format("2006-01-02")

	sinceDate := strings.TrimSpace(opts.Since)
	untilDate := strings.TrimSpace(opts.Until)
	if opts.Today {
		sinceDate = todayStr
		untilDate = todayStr
	} else if opts.Days > 0 {
		sinceDate = now.AddDate(0, 0, -(opts.Days - 1)).Format("2006-01-02")
	}

	surfFilter := opts.Surface
	if surfFilter == "" {
		surfFilter = model.SurfaceAll
	}
	projKW := strings.ToLower(strings.TrimSpace(opts.Project))
	modelKW := strings.ToLower(strings.TrimSpace(opts.Model))

	// Resolve root conversation ID across full conversation map for accurate project & subagent attribution.
	findGlobalRoot := func(cid string) string {
		visited := make(map[string]bool)
		cur := cid
		for !visited[cur] {
			visited[cur] = true
			c, ok := raw.Convs[cur]
			if !ok || c.ParentID == "" {
				return cur
			}
			if _, parentExists := raw.Convs[c.ParentID]; !parentExists {
				return cur
			}
			cur = c.ParentID
		}
		return cid
	}

	matchesProject := func(cid string) bool {
		if projKW == "" {
			return true
		}
		c := raw.Convs[cid]
		rootC := raw.Convs[findGlobalRoot(cid)]
		var sb strings.Builder
		if c != nil {
			sb.WriteString(c.Project)
			sb.WriteByte(' ')
			sb.WriteString(c.Workspace)
			sb.WriteByte(' ')
			sb.WriteString(c.Title)
		}
		if rootC != nil && rootC != c {
			sb.WriteByte(' ')
			sb.WriteString(rootC.Project)
			sb.WriteByte(' ')
			sb.WriteString(rootC.Workspace)
			sb.WriteByte(' ')
			sb.WriteString(rootC.Title)
		}
		return strings.Contains(strings.ToLower(sb.String()), projKW)
	}

	var filteredSteps []model.StepUsage
	for _, s := range raw.Steps {
		if sinceDate != "" && s.LocalDate < sinceDate {
			continue
		}
		if untilDate != "" && s.LocalDate > untilDate {
			continue
		}
		if (surfFilter == model.SurfaceIDE || surfFilter == model.SurfaceCLI) && s.Surface != surfFilter {
			continue
		}
		if !matchesProject(s.ConvID) {
			continue
		}
		if modelKW != "" && !strings.Contains(strings.ToLower(s.Model), modelKW) {
			continue
		}
		filteredSteps = append(filteredSteps, s)
	}

	filteredConvs := make(map[string]*model.ConvStats)
	for _, s := range filteredSteps {
		orig := raw.Convs[s.ConvID]
		fc, ok := filteredConvs[s.ConvID]
		if !ok {
			fc = &model.ConvStats{
				ConvID:       orig.ConvID,
				Title:        orig.Title,
				Surface:      orig.Surface,
				Project:      orig.Project,
				Workspace:    orig.Workspace,
				ParentID:     orig.ParentID,
				NestingDepth: orig.NestingDepth,
				Models:       make(map[string]struct{}),
			}
			filteredConvs[s.ConvID] = fc
		}
		fc.Steps++
		if s.StepType == "CHECKPOINT" {
			fc.Compactions++
		}
		fc.UncachedInput += s.UncachedInput
		fc.CacheRead += s.CacheRead
		fc.Output += s.Output
		if !strings.HasPrefix(s.Model, "[") {
			fc.Models[s.Model] = struct{}{}
		}
		if fc.FirstTS.IsZero() || s.Timestamp.Before(fc.FirstTS) {
			fc.FirstTS = s.Timestamp
		}
		if fc.LastTS.IsZero() || s.Timestamp.After(fc.LastTS) {
			fc.LastTS = s.Timestamp
		}
	}

	// Retain 0-step conversations when no date/model filter excludes them.
	if sinceDate == "" && untilDate == "" && modelKW == "" {
		for cid, orig := range raw.Convs {
			if _, exists := filteredConvs[cid]; exists {
				continue
			}
			if (surfFilter == model.SurfaceIDE || surfFilter == model.SurfaceCLI) && orig.Surface != surfFilter {
				continue
			}
			if !matchesProject(cid) {
				continue
			}
			filteredConvs[cid] = &model.ConvStats{
				ConvID:       orig.ConvID,
				Title:        orig.Title,
				Surface:      orig.Surface,
				Project:      orig.Project,
				Workspace:    orig.Workspace,
				ParentID:     orig.ParentID,
				NestingDepth: orig.NestingDepth,
				FirstTS:      orig.FirstTS,
				LastTS:       orig.LastTS,
				Models:       make(map[string]struct{}),
			}
		}
	}

	findFilteredRoot := func(cid string) string {
		visited := make(map[string]bool)
		cur := cid
		for !visited[cur] {
			visited[cur] = true
			c, ok := raw.Convs[cur]
			if !ok || c.ParentID == "" {
				return cur
			}
			if _, inFiltered := filteredConvs[c.ParentID]; !inFiltered {
				return cur
			}
			cur = c.ParentID
		}
		return cid
	}

	for _, fc := range filteredConvs {
		fc.RollupSteps = fc.Steps
		fc.RollupUncachedInput = fc.UncachedInput
		fc.RollupCacheRead = fc.CacheRead
		fc.RollupOutput = fc.Output
		fc.RollupLastTS = fc.LastTS
	}
	for cid, fc := range filteredConvs {
		rootID := findFilteredRoot(cid)
		if rootID != cid {
			if rc, ok := filteredConvs[rootID]; ok {
				rc.SubagentCount++
				rc.RollupSteps += fc.Steps
				rc.RollupUncachedInput += fc.UncachedInput
				rc.RollupCacheRead += fc.CacheRead
				rc.RollupOutput += fc.Output
				if !fc.LastTS.IsZero() && (rc.RollupLastTS.IsZero() || fc.LastTS.After(rc.RollupLastTS)) {
					rc.RollupLastTS = fc.LastTS
				}
			}
		}
	}

	var tot model.TotalsSummary
	bySurface := map[model.Surface]model.BucketMetrics{
		model.SurfaceIDE: {Key: string(model.SurfaceIDE)},
		model.SurfaceCLI: {Key: string(model.SurfaceCLI)},
	}

	for _, fc := range filteredConvs {
		tot.Conversations++
		b := bySurface[fc.Surface]
		b.Convs++
		if fc.IsSubagent() {
			tot.SubagentConversations++
			b.SubConvs++
		} else {
			tot.MainConversations++
			b.MainConvs++
		}
		bySurface[fc.Surface] = b
	}

	dateMap := make(map[string]*model.BucketMetrics)
	dateConvs := make(map[string]map[string]struct{})
	modelMap := make(map[string]*model.BucketMetrics)
	projMap := make(map[string]*model.BucketMetrics)
	projConvs := make(map[string]map[string]struct{})

	for _, s := range filteredSteps {
		tot.Steps++
		if s.StepType == "CHECKPOINT" {
			tot.Compactions++
		}
		tot.UncachedInput += s.UncachedInput
		tot.CacheRead += s.CacheRead
		tot.Output += s.Output
		if s.IsSubagent {
			tot.SubagentSteps++
			tot.SubagentTokens += s.TotalTokens()
		}

		sb := bySurface[s.Surface]
		sb.Steps++
		sb.Uncached += s.UncachedInput
		sb.Cache += s.CacheRead
		sb.Output += s.Output
		sb.Net += s.NetTokens()
		sb.Total += s.TotalTokens()
		bySurface[s.Surface] = sb

		dm, ok := dateMap[s.LocalDate]
		if !ok {
			dm = &model.BucketMetrics{Key: s.LocalDate}
			dateMap[s.LocalDate] = dm
			dateConvs[s.LocalDate] = make(map[string]struct{})
		}
		dm.Steps++
		dm.Uncached += s.UncachedInput
		dm.Cache += s.CacheRead
		dm.Output += s.Output
		dm.Net += s.NetTokens()
		dm.Total += s.TotalTokens()
		dateConvs[s.LocalDate][s.ConvID] = struct{}{}

		mm, ok := modelMap[s.Model]
		if !ok {
			mm = &model.BucketMetrics{Key: s.Model}
			modelMap[s.Model] = mm
		}
		mm.Steps++
		mm.Uncached += s.UncachedInput
		mm.Cache += s.CacheRead
		mm.Output += s.Output
		mm.Net += s.NetTokens()
		mm.Total += s.TotalTokens()

		rootID := findGlobalRoot(s.ConvID)
		projName := s.Project
		if rc, ok := raw.Convs[rootID]; ok && rc.Project != "" {
			projName = rc.Project
		}
		pm, ok := projMap[projName]
		if !ok {
			pm = &model.BucketMetrics{Key: projName}
			projMap[projName] = pm
			projConvs[projName] = make(map[string]struct{})
		}
		pm.Steps++
		pm.Uncached += s.UncachedInput
		pm.Cache += s.CacheRead
		pm.Output += s.Output
		pm.Net += s.NetTokens()
		pm.Total += s.TotalTokens()
		projConvs[projName][rootID] = struct{}{}
	}

	tot.TotalInput = tot.UncachedInput + tot.CacheRead
	tot.NetTokens = tot.UncachedInput + tot.Output
	tot.TotalTokens = tot.TotalInput + tot.Output
	if tot.TotalInput > 0 {
		tot.CacheHitRate = float64(tot.CacheRead) / float64(tot.TotalInput)
	}
	tot.ActiveDays = len(dateMap)

	byDate := make([]model.BucketMetrics, 0, len(dateMap))
	for d, bm := range dateMap {
		bm.Convs = len(dateConvs[d])
		byDate = append(byDate, *bm)
	}
	sort.Slice(byDate, func(i, j int) bool { return byDate[i].Key < byDate[j].Key })

	byModel := make([]model.BucketMetrics, 0, len(modelMap))
	for _, bm := range modelMap {
		byModel = append(byModel, *bm)
	}
	sortBucketsDesc(byModel)

	byProject := make([]model.BucketMetrics, 0, len(projMap))
	for p, bm := range projMap {
		bm.Convs = len(projConvs[p])
		byProject = append(byProject, *bm)
	}
	sortBucketsDesc(byProject)

	var sessions []model.SessionView
	for cid, fc := range filteredConvs {
		if !opts.Flat && findFilteredRoot(cid) != cid {
			continue
		}
		modelsList := make([]string, 0, len(fc.Models))
		for m := range fc.Models {
			modelsList = append(modelsList, m)
		}
		sort.Strings(modelsList)

		sv := model.SessionView{
			ConversationID: fc.ConvID,
			Title:          fc.Title,
			Surface:        fc.Surface,
			Project:        fc.Project,
			Workspace:      fc.Workspace,
			IsSubagent:     fc.IsSubagent(),
			SubagentCount:  fc.SubagentCount,
			OwnSteps:       fc.Steps,
			Models:         modelsList,
		}
		var activeTS time.Time
		if opts.Flat {
			sv.Steps = fc.Steps
			sv.UncachedInput = fc.UncachedInput
			sv.CacheRead = fc.CacheRead
			sv.Output = fc.Output
			sv.NetTokens = fc.NetTokens()
			sv.TotalTokens = fc.TotalTokens()
			activeTS = fc.LastTS
		} else {
			sv.Steps = fc.RollupSteps
			sv.UncachedInput = fc.RollupUncachedInput
			sv.CacheRead = fc.RollupCacheRead
			sv.Output = fc.RollupOutput
			sv.NetTokens = fc.RollupNetTokens()
			sv.TotalTokens = fc.RollupTotalTokens()
			activeTS = fc.RollupLastTS
		}
		if !activeTS.IsZero() {
			sv.LastActive = activeTS.In(loc).Format(time.RFC3339)
			sv.LastActiveUnix = activeTS.UnixNano()
		}
		sessions = append(sessions, sv)
	}

	sortBy := strings.ToLower(opts.SortBy)
	sort.Slice(sessions, func(i, j int) bool {
		switch sortBy {
		case "recent":
			if sessions[i].LastActiveUnix != sessions[j].LastActiveUnix {
				return sessions[i].LastActiveUnix > sessions[j].LastActiveUnix
			}
		case "steps":
			if sessions[i].Steps != sessions[j].Steps {
				return sessions[i].Steps > sessions[j].Steps
			}
		default:
			if sessions[i].TotalTokens != sessions[j].TotalTokens {
				return sessions[i].TotalTokens > sessions[j].TotalTokens
			}
		}
		return sessions[i].ConversationID < sessions[j].ConversationID
	})

	inaccSteps := 0
	for _, m := range raw.Inaccessible {
		inaccSteps += m.SummarySteps
	}

	return model.Report{
		GeneratedAt: now.In(loc).Format("2006-01-02T15:04:05-07:00"),
		Today:       todayStr,
		Filters: model.FilterEcho{
			Since:   sinceDate,
			Until:   untilDate,
			Surface: surfFilter,
			Project: opts.Project,
			Model:   opts.Model,
		},
		Totals:                    tot,
		BySurface:                 bySurface,
		ByDate:                    byDate,
		ByModel:                   byModel,
		ByProject:                 byProject,
		Sessions:                  sessions,
		InaccessibleConversations: len(raw.Inaccessible),
		InaccessibleSteps:         inaccSteps,
	}
}

func sortBucketsDesc(b []model.BucketMetrics) {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Total != b[j].Total {
			return b[i].Total > b[j].Total
		}
		return b[i].Key < b[j].Key
	})
}
