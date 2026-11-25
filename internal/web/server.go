package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LiangNing7/agy-usage/internal/collector"
	"github.com/LiangNing7/agy-usage/internal/model"
	"github.com/LiangNing7/agy-usage/internal/stats"
)

//go:embed index.html
var indexHTML []byte

// Config configures the local dashboard HTTP server.
type Config struct {
	AgyDir      string
	CLIDir      string
	Host        string
	Port        int
	OpenBrowser bool
	Location    *time.Location
}

// Handler returns the HTTP mux for serving the web dashboard and JSON API.
func Handler(agyDir, cliDir string, loc *time.Location) http.Handler {
	if loc == nil {
		loc = time.Local
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/usage", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		surfStr := strings.ToLower(strings.TrimSpace(q.Get("surface")))
		surf := model.SurfaceAll
		if surfStr == "ide" {
			surf = model.SurfaceIDE
		} else if surfStr == "cli" {
			surf = model.SurfaceCLI
		}
		days, _ := strconv.Atoi(q.Get("days"))
		flat := q.Get("flat") == "1" || strings.EqualFold(q.Get("flat"), "true")
		proj := strings.TrimSpace(q.Get("q"))
		modelKW := strings.TrimSpace(q.Get("model"))

		opts := model.FilterOptions{
			Today:   days == 1,
			Days:    days,
			Surface: surf,
			Project: proj,
			Model:   modelKW,
			Flat:    flat,
			SortBy:  "tokens",
			TopN:    100,
		}
		if days == 1 {
			opts.Days = 0
		}

		res, err := collector.Collect(agyDir, cliDir, loc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rep := stats.Aggregate(res, opts, time.Now().In(loc))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(rep)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	return mux
}

// Serve starts the local HTTP server and shuts down cleanly on SIGINT/SIGTERM.
func Serve(cfg Config) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           Handler(cfg.AgyDir, cfg.CLIDir, cfg.Location),
		ReadHeaderTimeout: 5 * time.Second,
	}

	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("◆ Antigravity Web Token Dashboard listening at %s (Press Ctrl+C to stop)\n", url)
	if cfg.OpenBrowser {
		openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		fmt.Println("\nStopped Antigravity Web Dashboard.")
		return nil
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
