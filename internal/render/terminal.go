package render

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/LiangNing7/agy-usage/internal/model"
)

// Lang specifies dashboard UI language.
type Lang string

const (
	LangAuto Lang = "auto"
	LangZH   Lang = "zh"
	LangEN   Lang = "en"
)

// ResolveLang resolves LangAuto using environment locale variables.
func ResolveLang(l Lang) Lang {
	switch strings.ToLower(string(l)) {
	case "zh", "zh-cn", "zh_cn", "cn":
		return LangZH
	case "en", "en-us", "en_us":
		return LangEN
	}
	for _, k := range []string{"AGY_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(os.Getenv(k))
		if v != "" {
			if strings.Contains(v, "zh") {
				return LangZH
			}
			return LangEN
		}
	}
	return LangEN
}

// Options configures CLI terminal rendering.
type Options struct {
	Color bool
	Raw   bool
	Flat  bool
	TopN  int
	Sort  string
	Width int // 0 = auto-detect
	Lang  Lang
}

// Style wraps ANSI color helpers.
type Style struct {
	Enabled bool
}

func (s Style) c(code, text string) string {
	if !s.Enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s Style) Bold(t string) string          { return s.c("1", t) }
func (s Style) Dim(t string) string           { return s.c("2", t) }
func (s Style) Cyan(t string) string          { return s.c("36", t) }
func (s Style) BrightCyan(t string) string    { return s.c("1;36", t) }
func (s Style) Green(t string) string         { return s.c("32", t) }
func (s Style) BrightGreen(t string) string   { return s.c("1;32", t) }
func (s Style) Yellow(t string) string        { return s.c("33", t) }
func (s Style) BrightYellow(t string) string  { return s.c("1;33", t) }
func (s Style) BrightMagenta(t string) string { return s.c("1;35", t) }
func (s Style) White(t string) string         { return s.c("1;37", t) }

type labels struct {
	emptyMsg       string
	kpiTotal       string
	kpiNet         string
	kpiCache       string
	kpiHitRate     string
	kpiOut         string
	kpiShare       string
	kpiConvs       string
	kpiMainSubFmt  string
	kpiSteps       string
	kpiCompactFmt  string
	kpiSubTok      string
	kpiSubShareFmt string
	kpiDailyAvg    string
	kpiActiveFmt   string
	secSurface     string
	surfTotal      string
	surfNet        string
	surfConvStep   string
	secDaily       string
	legNet         string
	legCache       string
	colDate        string
	colSteps       string
	colUncached    string
	colCache       string
	colOut         string
	colDayTotal    string
	colTrend       string
	secModel       string
	colModel       string
	colHit         string
	colTotal       string
	colShare       string
	secProj        string
	colProj        string
	colMainConvs   string
	colNet         string
	secSessFmt     string
	modeFlat       string
	modeRollup     string
	sortTokens     string
	sortRecent     string
	sortSteps      string
	colSurf        string
	colTitle       string
	colProjShort   string
	colNetShort    string
	colTotShort    string
	colLastActive  string
	subTagManyFmt  string
	subTagSingle   string
	sandboxWarnFmt string
	footerHints    string
}

func getLabels(lang Lang) labels {
	if lang == LangZH {
		return labels{
			emptyMsg:       "未找到符合当前筛选条件的 Antigravity 会话记录。",
			kpiTotal:       "总消耗 (含缓存)",
			kpiNet:         "非缓存净消耗",
			kpiCache:       "缓存读取 (Cache)",
			kpiHitRate:     "命中率 %s",
			kpiOut:         "模型总输出",
			kpiShare:       "占比 %s",
			kpiConvs:       "会话总数",
			kpiMainSubFmt:  "主 %d · 子代理 %d",
			kpiSteps:       "模型调用步数",
			kpiCompactFmt:  "自动压缩 %d 次",
			kpiSubTok:      "子智能体消耗",
			kpiSubShareFmt: "占比 %s (%d步)",
			kpiDailyAvg:    "日均总吞吐",
			kpiActiveFmt:   "活跃 %d 天",
			secSurface:     "▸ 端侧分布 (Surface: IDE/App vs CLI)",
			surfTotal:      "总",
			surfNet:        "净",
			surfConvStep:   "%2d会话/%4d步",
			secDaily:       "▸ 每日消耗趋势 (Daily Trend)",
			legNet:         "非缓存净消耗",
			legCache:       "缓存读取",
			colDate:        "日期",
			colSteps:       "步数",
			colUncached:    "未缓存In",
			colCache:       "缓存Read",
			colOut:         "输出Out",
			colDayTotal:    "当日总计",
			colTrend:       "趋势",
			secModel:       "▸ 模型消耗分布 (Model Breakdown)",
			colModel:       "模型名称",
			colHit:         "缓存率",
			colTotal:       "总计",
			colShare:       "占比",
			secProj:        "▸ 工作区 / 项目排行 (Workspaces)",
			colProj:        "项目 / 目录",
			colMainConvs:   "主会话",
			colNet:         "非缓存净计",
			secSessFmt:     "▸ 会话排行 Top %d",
			modeFlat:       "展开单会话",
			modeRollup:     "已聚合子代理开销",
			sortTokens:     "按总Token",
			sortRecent:     "按最近活跃",
			sortSteps:      "按步数",
			colSurf:        "端",
			colTitle:       "会话标题 / ID",
			colProjShort:   "项目",
			colNetShort:    "净Tokens",
			colTotShort:    "总Tokens",
			colLastActive:  "最后活跃",
			subTagManyFmt:  " [+%d子]",
			subTagSingle:   " [子]",
			sandboxWarnFmt: "[!] 检测到 %d 个历史会话当前处于沙盒隔离（共 %d 步）；在系统终端直接运行 agy-usage 可加载全部记录。",
			footerHints:    "用法: agy-usage [-t 今日] [-d 7 近7天] [-w 实时盯盘] [--web 网页看板] [--sort recent] [--flat]",
		}
	}
	return labels{
		emptyMsg:       "No Antigravity conversation records matched the current filters.",
		kpiTotal:       "Total Tokens (w/ Cache)",
		kpiNet:         "Uncached Net Tokens",
		kpiCache:       "Cache Read Tokens",
		kpiHitRate:     "Hit Rate %s",
		kpiOut:         "Total Output",
		kpiShare:       "Share %s",
		kpiConvs:       "Conversations",
		kpiMainSubFmt:  "Main %d · Sub %d",
		kpiSteps:       "Model Steps",
		kpiCompactFmt:  "Compactions: %d",
		kpiSubTok:      "Subagent Tokens",
		kpiSubShareFmt: "Share %s (%d steps)",
		kpiDailyAvg:    "Daily Avg Total",
		kpiActiveFmt:   "Active %d days",
		secSurface:     "▸ Surface Breakdown (IDE/App vs CLI)",
		surfTotal:      "Tot",
		surfNet:        "Net",
		surfConvStep:   "%2d conv/%4d st",
		secDaily:       "▸ Daily Token Trend",
		legNet:         "Uncached Net",
		legCache:       "Cache Read",
		colDate:        "Date",
		colSteps:       "Steps",
		colUncached:    "Uncached",
		colCache:       "CacheRd",
		colOut:         "Output",
		colDayTotal:    "Total",
		colTrend:       "Trend",
		secModel:       "▸ Model Breakdown",
		colModel:       "Model",
		colHit:         "Hit%",
		colTotal:       "Total",
		colShare:       "Share",
		secProj:        "▸ Workspaces / Projects",
		colProj:        "Project",
		colMainConvs:   "Convs",
		colNet:         "NetTokens",
		secSessFmt:     "▸ Top %d Sessions",
		modeFlat:       "flat view",
		modeRollup:     "subagents rolled up",
		sortTokens:     "by total tokens",
		sortRecent:     "by recent activity",
		sortSteps:      "by steps",
		colSurf:        "Src",
		colTitle:       "Session Title / ID",
		colProjShort:   "Project",
		colNetShort:    "NetTok",
		colTotShort:    "TotalTok",
		colLastActive:  "Last Active",
		subTagManyFmt:  " [+%d sub]",
		subTagSingle:   " [sub]",
		sandboxWarnFmt: "[!] %d historical conversation(s) (%d steps) are hidden by sandbox mount; run agy-usage in host terminal for full stats.",
		footerHints:    "Tips: agy-usage [-t today] [-d 7 days] [-w watch] [--web UI] [--sort recent] [--flat] [--lang zh|en]",
	}
}

// RenderTerminal renders the boxed CLI dashboard with exact East-Asian column alignment.
func RenderTerminal(rep model.Report, opts Options) string {
	lang := ResolveLang(opts.Lang)
	L := getLabels(lang)
	st := Style{Enabled: opts.Color}

	termW := opts.Width
	if termW <= 0 {
		termW = DetectTerminalWidth(112)
	}
	width := termW
	if width < 86 {
		width = 86
	}
	if width > 128 {
		width = 128
	}
	innerW := width - 4
	raw := opts.Raw

	var lines []string
	hr := func(left, mid, right string) string {
		return st.Dim(left + strings.Repeat(mid, width-2) + right)
	}
	boxRow := func(content string) string {
		if DispWidth(content) > innerW {
			content = TruncateDisp(StripANSI(content), innerW)
		}
		return st.Dim("│") + " " + PadRight(content, innerW) + " " + st.Dim("│")
	}

	// Banner
	titleLeft := st.BrightCyan("◆ ANTIGRAVITY TOKEN DASHBOARD")
	var tags []string
	if rep.Filters.Since != "" || rep.Filters.Until != "" {
		s := rep.Filters.Since
		if s == "" {
			s = ".."
		}
		u := rep.Filters.Until
		if u == "" {
			u = "now"
		}
		if s == u {
			tags = append(tags, s)
		} else {
			tags = append(tags, s+"->"+u)
		}
	}
	if rep.Filters.Surface != "" && rep.Filters.Surface != model.SurfaceAll {
		tags = append(tags, "surface:"+strings.ToUpper(string(rep.Filters.Surface)))
	}
	if rep.Filters.Project != "" {
		tags = append(tags, "proj:"+rep.Filters.Project)
	}
	if rep.Filters.Model != "" {
		tags = append(tags, "model:"+rep.Filters.Model)
	}
	filterStr := " [ALL TIME]"
	if len(tags) > 0 {
		filterStr = " [" + strings.Join(tags, ", ") + "]"
	}
	tsDisp := rep.GeneratedAt
	if len(tsDisp) >= 19 {
		tsDisp = strings.Replace(tsDisp[:19], "T", " ", 1)
	}
	rightTS := st.Dim(tsDisp)
	leftPart := titleLeft + st.Yellow(filterStr)
	gap := innerW - DispWidth(leftPart) - DispWidth(rightTS)
	if gap < 1 {
		gap = 1
	}

	lines = append(lines, st.Dim("╭"+strings.Repeat("─", width-2)+"╮"))
	lines = append(lines, boxRow(leftPart+strings.Repeat(" ", gap)+rightTS))
	lines = append(lines, hr("├", "─", "┤"))

	tot := rep.Totals
	if tot.Steps == 0 && tot.Conversations == 0 {
		lines = append(lines, boxRow(st.Yellow(L.emptyMsg)))
		lines = append(lines, st.Dim("╰"+strings.Repeat("─", width-2)+"╯"))
		return strings.Join(lines, "\n")
	}

	// 1. KPI Grid (2 rows x 4 cols)
	activeDays := tot.ActiveDays
	if activeDays < 1 {
		activeDays = 1
	}
	dailyAvg := tot.TotalTokens / int64(activeDays)
	hitPct := FmtPct(tot.CacheRead, tot.TotalInput)

	baseColW := (innerW - 9) / 4
	colWidths := []int{baseColW, baseColW, baseColW, innerW - 9 - 3*baseColW}

	type kpiItem struct {
		lbl string
		val string
		sub string
	}
	row1 := []kpiItem{
		{L.kpiTotal, st.BrightCyan(FmtNum(tot.TotalTokens, raw)), st.Dim(FmtComma(tot.TotalTokens))},
		{L.kpiNet, st.BrightGreen(FmtNum(tot.NetTokens, raw)), st.Dim(fmt.Sprintf("In %s+Out %s", FmtNum(tot.UncachedInput, false), FmtNum(tot.Output, false)))},
		{L.kpiCache, st.Cyan(FmtNum(tot.CacheRead, raw)), st.BrightYellow(fmt.Sprintf(L.kpiHitRate, hitPct))},
		{L.kpiOut, st.BrightMagenta(FmtNum(tot.Output, raw)), st.Dim(fmt.Sprintf(L.kpiShare, FmtPct(tot.Output, tot.TotalTokens)))},
	}
	row2 := []kpiItem{
		{L.kpiConvs, st.White(fmt.Sprintf("%d", tot.Conversations)), st.Dim(fmt.Sprintf(L.kpiMainSubFmt, tot.MainConversations, tot.SubagentConversations))},
		{L.kpiSteps, st.White(FmtComma(int64(tot.Steps))), st.Dim(fmt.Sprintf(L.kpiCompactFmt, tot.Compactions))},
		{L.kpiSubTok, st.Yellow(FmtNum(tot.SubagentTokens, raw)), st.Dim(fmt.Sprintf(L.kpiSubShareFmt, FmtPct(tot.SubagentTokens, tot.TotalTokens), tot.SubagentSteps))},
		{L.kpiDailyAvg, st.White(FmtNum(dailyAvg, raw)), st.Dim(fmt.Sprintf(L.kpiActiveFmt, tot.ActiveDays))},
	}

	sep := " " + st.Dim("│") + " "
	for idx, rItems := range [][]kpiItem{row1, row2} {
		var lbls, vals, subs []string
		for i, k := range rItems {
			cw := colWidths[i]
			lbls = append(lbls, FitCell(st.Dim(TruncateDisp(k.lbl, cw)), cw, false))
			vals = append(vals, FitCell(k.val, cw, false))
			subs = append(subs, FitCell(k.sub, cw, false))
		}
		lines = append(lines, boxRow(strings.Join(lbls, sep)))
		lines = append(lines, boxRow(strings.Join(vals, sep)))
		lines = append(lines, boxRow(strings.Join(subs, sep)))
		if idx == 0 {
			lines = append(lines, boxRow(st.Dim(strings.Repeat("─", innerW))))
		}
	}

	// 2. Surface Comparison
	lines = append(lines, hr("├", "─", "┤"))
	lines = append(lines, boxRow(st.Bold(L.secSurface)))
	for _, sItem := range []struct {
		key   model.Surface
		label string
		ide   bool
	}{
		{model.SurfaceIDE, "Antigravity IDE", true},
		{model.SurfaceCLI, "Antigravity CLI", false},
	} {
		sb := rep.BySurface[sItem.key]
		ratio := 0.0
		if tot.TotalTokens > 0 {
			ratio = float64(sb.Total) / float64(tot.TotalTokens)
		}
		var badge string
		var barColor func(string) string
		if sItem.ide {
			badge = st.BrightCyan(sItem.label)
			barColor = st.BrightCyan
		} else {
			badge = st.BrightMagenta(sItem.label)
			barColor = st.BrightMagenta
		}
		prefix := badge + "  "
		suffix := fmt.Sprintf(" %s %s %s %s %s %s %s %s %s",
			PadLeft(FmtPct(sb.Total, tot.TotalTokens), 6),
			st.Dim("│"),
			L.surfTotal,
			PadLeft(st.Bold(FmtNum(sb.Total, raw)), 8),
			st.Dim("│"),
			L.surfNet,
			PadLeft(FmtNum(sb.Net, raw), 8),
			st.Dim("│"),
			fmt.Sprintf(L.surfConvStep, sb.Convs, sb.Steps),
		)
		barW := innerW - DispWidth(prefix) - DispWidth(suffix)
		if barW < 4 {
			barW = 4
		}
		lines = append(lines, boxRow(prefix+renderBar(ratio, barW, st, barColor)+suffix))
	}

	// 3. Daily Trend
	if len(rep.ByDate) > 0 {
		lines = append(lines, hr("├", "─", "┤"))
		legend := fmt.Sprintf("%s %s  %s %s", st.BrightGreen("█"), L.legNet, st.Cyan("▒"), L.legCache)
		hdrL := st.Bold(L.secDaily)
		gapD := innerW - DispWidth(hdrL) - DispWidth(legend)
		if gapD < 2 {
			gapD = 2
		}
		lines = append(lines, boxRow(hdrL+strings.Repeat(" ", gapD)+legend))

		var maxDay int64 = 1
		for _, dv := range rep.ByDate {
			if dv.Total > maxDay {
				maxDay = dv.Total
			}
		}
		colHdrPrefix := fmt.Sprintf("%s  %s  %s  %s  %s  %s  ",
			PadRight(L.colDate, 11),
			PadLeft(L.colSteps, 5),
			PadLeft(L.colUncached, 9),
			PadLeft(L.colCache, 9),
			PadLeft(L.colOut, 8),
			PadLeft(L.colDayTotal, 9),
		)
		dayBarW := innerW - DispWidth(colHdrPrefix)
		if dayBarW < 6 {
			dayBarW = 6
		}
		lines = append(lines, boxRow(st.Dim(colHdrPrefix+L.colTrend)))

		daysSlice := rep.ByDate
		if len(daysSlice) > 14 {
			daysSlice = daysSlice[len(daysSlice)-14:]
		}
		for _, dv := range daysSlice {
			isToday := dv.Key == rep.Today
			dayDisp := dv.Key
			if isToday {
				dayDisp += "*"
			}
			dayCol := PadRight(dayDisp, 11)
			if isToday {
				dayCol = st.BrightYellow(dayCol)
			}
			rowPrefix := fmt.Sprintf("%s  %s  %s  %s  %s  %s  ",
				dayCol,
				PadLeft(fmt.Sprintf("%d", dv.Steps), 5),
				PadLeft(FmtNum(dv.Uncached, raw), 9),
				PadLeft(FmtNum(dv.Cache, raw), 9),
				PadLeft(FmtNum(dv.Output, raw), 8),
				st.Bold(PadLeft(FmtNum(dv.Total, raw), 9)),
			)
			sbar := renderStackedBar(
				float64(dv.Cache)/float64(maxDay),
				float64(dv.Net)/float64(maxDay),
				dayBarW,
				st,
			)
			lines = append(lines, boxRow(rowPrefix+sbar))
		}
	}

	// 4. Model Breakdown
	if len(rep.ByModel) > 0 {
		lines = append(lines, hr("├", "─", "┤"))
		lines = append(lines, boxRow(st.Bold(L.secModel)))
		numW := 8
		outW := 7
		if raw {
			numW = 10
			outW = 9
		}
		fixedM := 1 + 5 + 2 + numW + 2 + numW + 2 + outW + 2 + 6 + 2 + numW + 2 + 7
		mNameW := innerW - fixedM - 6
		if mNameW < 15 {
			mNameW = 15
		}
		if mNameW > 28 {
			mNameW = 28
		}
		mHdrPrefix := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  ",
			PadRight(L.colModel, mNameW),
			PadLeft(L.colSteps, 5),
			PadLeft(L.colUncached, numW),
			PadLeft(L.colCache, numW),
			PadLeft(L.colOut, outW),
			PadLeft(L.colHit, 6),
			PadLeft(L.colTotal, numW),
		)
		mBarW := innerW - DispWidth(mHdrPrefix) - 7
		if mBarW < 3 {
			mBarW = 3
		}
		lines = append(lines, boxRow(st.Dim(mHdrPrefix+L.colShare)))
		for _, mv := range rep.ByModel {
			mDisp := TruncateDisp(mv.Key, mNameW)
			mIn := mv.Uncached + mv.Cache
			ratio := 0.0
			if tot.TotalTokens > 0 {
				ratio = float64(mv.Total) / float64(tot.TotalTokens)
			}
			mbar := renderBar(ratio, mBarW, st, st.Green)
			rowM := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s %s",
				PadRight(st.Cyan(mDisp), mNameW),
				PadLeft(fmt.Sprintf("%d", mv.Steps), 5),
				PadLeft(FmtNum(mv.Uncached, raw), numW),
				PadLeft(FmtNum(mv.Cache, raw), numW),
				PadLeft(FmtNum(mv.Output, raw), outW),
				PadLeft(FmtPct(mv.Cache, mIn), 6),
				st.Bold(PadLeft(FmtNum(mv.Total, raw), numW)),
				mbar,
				PadLeft(FmtPct(mv.Total, tot.TotalTokens), 6),
			)
			lines = append(lines, boxRow(rowM))
		}
	}

	// 5. Project Breakdown
	if len(rep.ByProject) > 0 {
		lines = append(lines, hr("├", "─", "┤"))
		lines = append(lines, boxRow(st.Bold(L.secProj)))
		pNumW := 9
		if raw {
			pNumW = 11
		}
		fixedP := 2 + 6 + 2 + 5 + 2 + pNumW + 2 + pNumW + 2 + pNumW + 2 + 7
		pNameW := innerW - fixedP - 8
		if pNameW < 14 {
			pNameW = 14
		}
		if pNameW > 26 {
			pNameW = 26
		}
		pHdrPrefix := fmt.Sprintf("%s  %s  %s  %s  %s  %s  ",
			PadRight(L.colProj, pNameW),
			PadLeft(L.colMainConvs, 6),
			PadLeft(L.colSteps, 5),
			PadLeft(L.colNet, pNumW),
			PadLeft(L.colCache, pNumW),
			PadLeft(L.colTotal, pNumW),
		)
		pBarW := innerW - DispWidth(pHdrPrefix) - 7
		if pBarW < 4 {
			pBarW = 4
		}
		lines = append(lines, boxRow(st.Dim(pHdrPrefix+L.colShare)))
		limit := len(rep.ByProject)
		if limit > 8 {
			limit = 8
		}
		for _, pv := range rep.ByProject[:limit] {
			pDisp := TruncateDisp(pv.Key, pNameW)
			ratio := 0.0
			if tot.TotalTokens > 0 {
				ratio = float64(pv.Total) / float64(tot.TotalTokens)
			}
			pbar := renderBar(ratio, pBarW, st, st.Yellow)
			rowP := fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s %s",
				PadRight(st.White(pDisp), pNameW),
				PadLeft(fmt.Sprintf("%d", pv.Convs), 6),
				PadLeft(fmt.Sprintf("%d", pv.Steps), 5),
				PadLeft(FmtNum(pv.Net, raw), pNumW),
				PadLeft(FmtNum(pv.Cache, raw), pNumW),
				st.Bold(PadLeft(FmtNum(pv.Total, raw), pNumW)),
				pbar,
				PadLeft(FmtPct(pv.Total, tot.TotalTokens), 6),
			)
			lines = append(lines, boxRow(rowP))
		}
	}

	// 6. Top Sessions
	topN := opts.TopN
	if topN <= 0 {
		topN = 10
	}
	sessSlice := rep.Sessions
	if len(sessSlice) > topN {
		sessSlice = sessSlice[:topN]
	}
	if len(sessSlice) > 0 {
		lines = append(lines, hr("├", "─", "┤"))
		modeNote := L.modeRollup
		if opts.Flat {
			modeNote = L.modeFlat
		}
		sortNote := L.sortTokens
		switch strings.ToLower(opts.Sort) {
		case "recent":
			sortNote = L.sortRecent
		case "steps":
			sortNote = L.sortSteps
		}
		lines = append(lines, boxRow(fmt.Sprintf("%s %s",
			st.Bold(fmt.Sprintf(L.secSessFmt, len(sessSlice))),
			st.Dim(fmt.Sprintf("(%s · %s)", sortNote, modeNote)),
		)))

		tokW := 9
		if raw {
			tokW = 11
		}
		// Fixed cols: 3 + 2 + titleW + 2 + 12 + 2 + 6 + 2 + tokW + 2 + tokW + 2 + 11 = titleW + 44 + 2*tokW
		titleW := innerW - (44 + 2*tokW)
		if titleW < 14 {
			titleW = 14
		}
		sHdr := fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s",
			PadRight(L.colSurf, 3),
			PadRight(L.colTitle, titleW),
			PadRight(L.colProjShort, 12),
			PadLeft(L.colSteps, 6),
			PadLeft(L.colNetShort, tokW),
			PadLeft(L.colTotShort, tokW),
			PadLeft(L.colLastActive, 11),
		)
		lines = append(lines, boxRow(st.Dim(sHdr)))

		for _, c := range sessSlice {
			surfBadge := st.BrightCyan("IDE")
			if c.Surface == model.SurfaceCLI {
				surfBadge = st.BrightMagenta("CLI")
			}
			subTag := ""
			if !opts.Flat && c.SubagentCount > 0 {
				subTag = fmt.Sprintf(L.subTagManyFmt, c.SubagentCount)
			} else if opts.Flat && c.IsSubagent {
				subTag = L.subTagSingle
			}

			shortID := c.ConversationID
			if len(shortID) > 6 {
				shortID = shortID[:6]
			}
			idPrefix := shortID + " "
			availT := titleW - DispWidth(idPrefix) - DispWidth(subTag)
			if availT < 4 {
				availT = 4
			}
			tClean := TruncateDisp(c.Title, availT)
			titleCell := st.Dim(idPrefix) + tClean
			if subTag != "" {
				titleCell += st.Yellow(subTag)
			}

			ltsStr := "-"
			if c.LastActive != "" {
				if t, err := time.Parse(time.RFC3339, c.LastActive); err == nil {
					ltsStr = t.Format("01-02 15:04")
				}
			}

			rowC := fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s",
				PadRight(surfBadge, 3),
				PadRight(titleCell, titleW),
				PadRight(TruncateDisp(c.Project, 12), 12),
				PadLeft(fmt.Sprintf("%d", c.Steps), 6),
				PadLeft(FmtNum(c.NetTokens, raw), tokW),
				st.Bold(PadLeft(FmtNum(c.TotalTokens, raw), tokW)),
				st.Dim(PadLeft(ltsStr, 11)),
			)
			lines = append(lines, boxRow(rowC))
		}
	}

	if rep.InaccessibleConversations > 0 {
		lines = append(lines, hr("├", "─", "┤"))
		warn := fmt.Sprintf(L.sandboxWarnFmt, rep.InaccessibleConversations, rep.InaccessibleSteps)
		lines = append(lines, boxRow(st.Yellow(TruncateDisp(warn, innerW))))
	}

	lines = append(lines, hr("├", "─", "┤"))
	lines = append(lines, boxRow(st.Dim(TruncateDisp(L.footerHints, innerW))))
	lines = append(lines, st.Dim("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(lines, "\n")
}

func renderBar(ratio float64, width int, st Style, colorFn func(string) string) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(math.Round(ratio * float64(width)))
	if filled > width {
		filled = width
	}
	empty := width - filled
	fStr := strings.Repeat("█", filled)
	if colorFn != nil {
		fStr = colorFn(fStr)
	}
	return fStr + st.Dim(strings.Repeat("░", empty))
}

func renderStackedBar(cacheRatio, netRatio float64, width int, st Style) string {
	sumR := cacheRatio + netRatio
	if sumR < 0 {
		sumR = 0
	}
	if sumR > 1 {
		sumR = 1
	}
	totalCells := int(math.Round(sumR * float64(width)))
	if totalCells == 0 && (cacheRatio+netRatio) > 0 {
		totalCells = 1
	}
	netCells := 0
	if cacheRatio+netRatio > 0 {
		netCells = int(math.Round((netRatio / (cacheRatio + netRatio)) * float64(totalCells)))
		if netRatio > 0 && netCells == 0 && totalCells >= 2 {
			netCells = 1
		}
	}
	cacheCells := totalCells - netCells
	if cacheCells < 0 {
		cacheCells = 0
	}
	emptyCells := width - cacheCells - netCells
	if emptyCells < 0 {
		emptyCells = 0
	}
	return st.BrightGreen(strings.Repeat("█", netCells)) +
		st.Cyan(strings.Repeat("▒", cacheCells)) +
		st.Dim(strings.Repeat("░", emptyCells))
}

// FmtNum formats token counts into K/M/B or comma-grouped integers when raw is true.
func FmtNum(n int64, raw bool) string {
	if raw {
		return FmtComma(n)
	}
	abs := math.Abs(float64(n))
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(n)/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FmtComma formats an int64 with thousands separators.
func FmtComma(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var out []byte
	rem := len(s) % 3
	if rem > 0 {
		out = append(out, s[:rem]...)
		if len(s) > rem {
			out = append(out, ',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, ',')
		}
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// FmtPct formats part/whole as a percentage string like "80.7%".
func FmtPct(part, whole int64) string {
	if whole <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(part)*100.0/float64(whole))
}
