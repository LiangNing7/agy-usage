package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LiangNing7/agy-usage/internal/collector"
	"github.com/LiangNing7/agy-usage/internal/model"
	"github.com/LiangNing7/agy-usage/internal/render"
	"github.com/LiangNing7/agy-usage/internal/stats"
	"github.com/LiangNing7/agy-usage/internal/version"
	"github.com/LiangNing7/agy-usage/internal/web"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(rawArgs []string) int {
	defaultAgyDir, defaultCLIDir := collector.DefaultPaths()

	fs := flag.NewFlagSet("agy-usage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		today       bool
		days        int
		since       string
		until       string
		surfaceStr  string
		project     string
		modelKW     string
		topN        int
		sortBy      string
		flat        bool
		rawNum      bool
		watchSec    float64
		webMode     bool
		host        string
		port        int
		noOpen      bool
		jsonOut     bool
		langStr     string
		forceColor  bool
		noColor     bool
		agyDir      string
		cliDir      string
		showVersion bool
	)

	fs.BoolVar(&today, "today", false, "Show today's token usage only")
	fs.BoolVar(&today, "t", false, "Shorthand for --today")
	fs.IntVar(&days, "days", 0, "Show usage from the last N days (e.g. -d 7)")
	fs.IntVar(&days, "d", 0, "Shorthand for --days")
	fs.StringVar(&since, "since", "", "Start date in local timezone (YYYY-MM-DD)")
	fs.StringVar(&until, "until", "", "End date in local timezone (YYYY-MM-DD)")
	fs.StringVar(&surfaceStr, "surface", "all", "Filter surface: all | ide | cli")
	fs.StringVar(&surfaceStr, "s", "all", "Shorthand for --surface")
	fs.StringVar(&project, "project", "", "Filter by workspace/project or session title keyword")
	fs.StringVar(&project, "p", "", "Shorthand for --project")
	fs.StringVar(&modelKW, "model", "", "Filter by model name substring")
	fs.StringVar(&modelKW, "m", "", "Shorthand for --model")
	fs.IntVar(&topN, "top", 10, "Show top N sessions")
	fs.StringVar(&sortBy, "sort", "tokens", "Sort sessions by: tokens | recent | steps")
	fs.BoolVar(&flat, "flat", false, "List subagent sessions individually without parent rollup")
	fs.BoolVar(&rawNum, "raw", false, "Display exact comma-separated integers instead of K/M/B")
	fs.Float64Var(&watchSec, "watch", 0, "Live auto-refresh interval in seconds (default 2s when -w is passed)")
	fs.Float64Var(&watchSec, "w", 0, "Shorthand for --watch")
	fs.BoolVar(&webMode, "web", false, "Start local interactive web dashboard")
	fs.StringVar(&host, "host", "127.0.0.1", "Web dashboard listen address")
	fs.IntVar(&port, "port", 8765, "Web dashboard listen port")
	fs.BoolVar(&noOpen, "no-open", false, "Do not auto-open browser in --web mode")
	fs.BoolVar(&jsonOut, "json", false, "Output structured JSON report")
	fs.StringVar(&langStr, "lang", "auto", "UI language: auto | zh | en")
	fs.BoolVar(&forceColor, "color", false, "Force ANSI color output")
	fs.BoolVar(&noColor, "no-color", false, "Disable ANSI color output")
	fs.StringVar(&agyDir, "agy-dir", defaultAgyDir, "Antigravity IDE/App data directory")
	fs.StringVar(&cliDir, "cli-dir", defaultCLIDir, "Antigravity CLI data directory")
	fs.BoolVar(&showVersion, "version", false, "Print version information")
	fs.BoolVar(&showVersion, "v", false, "Shorthand for --version")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `agy-usage - Antigravity Token Usage Dashboard (CLI & Web UI)

Usage:
  agy-usage [flags]

Examples:
  agy-usage                     Full terminal token dashboard
  agy-usage -t                  Today's usage only
  agy-usage -d 7                Last 7 days usage
  agy-usage -s cli              CLI conversations only (-s ide for IDE/App)
  agy-usage -p my-repo          Filter by workspace/project or session title
  agy-usage -w                  Live watch mode (refreshes every 2s; -w 5 for 5s)
  agy-usage --web               Launch local interactive web dashboard
  agy-usage --json              Export structured JSON report

Flags:
`)
		fs.PrintDefaults()
	}

	normArgs := normalizeArgs(rawArgs)
	if len(normArgs) == 1 && normArgs[0] == "version" {
		fmt.Println(version.String())
		return 0
	}
	if err := fs.Parse(normArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if showVersion {
		fmt.Println(version.String())
		return 0
	}

	surf := model.Surface(strings.ToLower(strings.TrimSpace(surfaceStr)))
	switch surf {
	case model.SurfaceAll, model.SurfaceIDE, model.SurfaceCLI:
	default:
		fmt.Fprintf(os.Stderr, "invalid --surface %q (expected: all, ide, or cli)\n", surfaceStr)
		return 2
	}

	switch strings.ToLower(sortBy) {
	case "tokens", "recent", "steps":
	default:
		fmt.Fprintf(os.Stderr, "invalid --sort %q (expected: tokens, recent, or steps)\n", sortBy)
		return 2
	}

	loc := time.Local
	if webMode {
		if err := web.Serve(web.Config{
			AgyDir:      agyDir,
			CLIDir:      cliDir,
			Host:        host,
			Port:        port,
			OpenBrowser: !noOpen,
			Location:    loc,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "web server error: %v\n", err)
			return 1
		}
		return 0
	}

	useColor := (isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" && !noColor) || forceColor

	fOpts := model.FilterOptions{
		Today:   today,
		Days:    days,
		Since:   since,
		Until:   until,
		Surface: surf,
		Project: project,
		Model:   modelKW,
		Flat:    flat,
		SortBy:  sortBy,
		TopN:    topN,
	}
	rOpts := render.Options{
		Color: useColor,
		Raw:   rawNum,
		Flat:  flat,
		TopN:  topN,
		Sort:  sortBy,
		Lang:  render.Lang(langStr),
	}

	if watchSec > 0 {
		interval := time.Duration(watchSec * float64(time.Second))
		if interval < 500*time.Millisecond {
			interval = 500 * time.Millisecond
		}
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		st := render.Style{Enabled: useColor}
		lang := render.ResolveLang(rOpts.Lang)

		for {
			res, err := collector.Collect(agyDir, cliDir, loc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "collect error: %v\n", err)
				return 1
			}
			rep := stats.Aggregate(res, fOpts, time.Now().In(loc))
			board := render.RenderTerminal(rep, rOpts)
			hint := fmt.Sprintf("Watching live (refresh every %.1fs, press Ctrl+C to quit)", interval.Seconds())
			if lang == render.LangZH {
				hint = fmt.Sprintf("实时盯盘中 (每 %.1fs 自动刷新，按 Ctrl+C 退出)", interval.Seconds())
			}
			fmt.Printf("\x1b[2J\x1b[H%s\n%s\n", board, st.Dim(hint))

			select {
			case <-sigCh:
				fmt.Println()
				return 0
			case <-time.After(interval):
			}
		}
	}

	res, err := collector.Collect(agyDir, cliDir, loc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect error: %v\n", err)
		return 1
	}
	rep := stats.Aggregate(res, fOpts, time.Now().In(loc))

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}

	fmt.Println(render.RenderTerminal(rep, rOpts))
	return 0
}

// normalizeArgs supports optional argument on -w/--watch (defaulting to 2s)
// and reorders flags before positional args for standard library flag compatibility.
func normalizeArgs(args []string) []string {
	var flagsPart []string
	var posPart []string

	takesValue := map[string]bool{
		"-d": true, "--days": true,
		"--since": true, "--until": true,
		"-s": true, "--surface": true,
		"-p": true, "--project": true,
		"-m": true, "--model": true,
		"--top": true, "--sort": true,
		"--host": true, "--port": true,
		"--lang":    true,
		"--agy-dir": true, "--cli-dir": true,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			posPart = append(posPart, args[i+1:]...)
			break
		}
		if a == "-w" || a == "--watch" {
			if i+1 < len(args) && isPositiveNumber(args[i+1]) {
				flagsPart = append(flagsPart, a, args[i+1])
				i++
			} else {
				flagsPart = append(flagsPart, a+"=2")
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			flagsPart = append(flagsPart, a)
			if !strings.Contains(a, "=") && takesValue[a] && i+1 < len(args) {
				flagsPart = append(flagsPart, args[i+1])
				i++
			}
			continue
		}
		posPart = append(posPart, a)
	}
	return append(flagsPart, posPart...)
}

func isPositiveNumber(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	v, err := strconv.ParseFloat(s, 64)
	return err == nil && v > 0
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}
