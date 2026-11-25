# agy-usage

**English** | [简体中文](./README.zh-CN.md)

A fast, zero-dependency CLI & local Web token usage dashboard for **Google Antigravity** (covering both **Antigravity IDE/App** and **Antigravity CLI**), inspired by Codex / `ccusage` token dashboards.

Built with **100% Go standard library (`CGO_ENABLED=0`)** — including an embedded read-only SQLite3 B-Tree + WAL parser — so it compiles in under a second on Linux, macOS, and Windows with zero external packages or C toolchains.

---

## Highlights

- **Unified Surface Coverage**: Automatically scans `~/.gemini/antigravity` (Desktop/IDE) and `~/.gemini/antigravity-cli` (CLI) across Linux, macOS, and Windows.
- **Accurate Token Semantics**: Distinguishes **Uncached Input**, **Cache Read** (with cache hit rate), **Output**, **Net Tokens** (`Uncached In + Out`), and **Total Throughput** (`Uncached In + Cache Read + Out`).
- **Subagent Tree Rollup**: Reads `parent_conversation_id` from `conversation_summaries.db` to roll up spawned subagent token usage into their parent conversation and workspace (`[+N sub]`), or view every subagent individually with `--flat`.
- **Context Compaction Tracking**: Detects system `CHECKPOINT` summarization steps and reports them separately under `[context-compaction]`.
- **Live Watch & Local Web UI**:
  - `agy-usage -w`: Flicker-free live terminal monitor while long agent tasks run.
  - `agy-usage --web`: Single-binary dark-mode interactive web dashboard (`http://127.0.0.1:8765`) with live auto-sync and search.
- **Bilingual (`--lang auto|en|zh`) & CJK-Safe Alignment**: Pixel-aligned Unicode box tables across terminal widths (86–128 cols).
- **100% Local & Read-Only**: Never modifies your Antigravity files and makes zero network requests.

---

## Terminal Preview

```text
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ◆ ANTIGRAVITY TOKEN DASHBOARD [ALL TIME]                                                 2026-11-25 16:30:47 │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Total Tokens (w/ Cache)  │ Uncached Net Tokens      │ Cache Read Tokens        │ Total Output                │
│ 737.30M                  │ 158.59M                  │ 578.71M                  │ 4.70M                       │
│ 737,297,908              │ In 153.89M+Out 4.70M     │ Hit Rate 79.0%           │ Share 0.6%                  │
│ ──────────────────────────────────────────────────────────────────────────────────────────────────────────── │
│ Conversations            │ Model Steps              │ Subagent Tokens          │ Daily Avg Total             │
│ 43                       │ 3,129                    │ 74.37M                   │ 184.32M                     │
│ Main 17 · Sub 26         │ Compactions: 17          │ Share 10.1% (487 steps)  │ Active 4 days               │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ▸ Daily Token Trend                                                              █ Uncached Net  ▒ Cache Read │
│ Date        Steps   Uncached    CacheRd    Output      Total  Trend                                          │
│ 2026-11-23   1125     98.40M    217.50M     2.10M    318.00M  ██████████████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒ │
│ 2026-11-24   1236     32.87M    182.77M     1.35M    216.99M  █████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░░░░░░░░░░░░░░░ │
│ 2026-11-25*   768     22.62M    178.44M     1.25M    202.31M  ████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░░░░░░░░░░░░░░░░░ │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

---

## Installation

### Option 1: `go install` (Recommended, Go 1.22+)

Zero external dependencies — builds instantaneously without CGO:

```bash
CGO_ENABLED=0 go install github.com/LiangNing7/agy-usage/cmd/agy-usage@latest
```

### Option 2: Quick Install Script (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/LiangNing7/agy-usage/main/install.sh | bash
```

### Option 3: Build from Source

```bash
git clone https://github.com/LiangNing7/agy-usage.git
cd agy-usage
make install   # installs to ~/.local/bin/agy-usage by default
```

*(On Windows, download `agy-usage_windows_amd64.zip` from Releases or run `go install github.com/LiangNing7/agy-usage/cmd/agy-usage@latest`.)*

---

## Usage

```bash
# Default full terminal dashboard (auto-detects system language)
agy-usage

# Live watch mode (refreshes every 2s by default, or specify custom seconds)
agy-usage -w
agy-usage -w 5

# Launch interactive local Web UI (http://127.0.0.1:8765)
agy-usage --web
agy-usage --web --port 9000 --no-open

# Time & dimension filters
agy-usage -t                        # Today only (--today)
agy-usage -d 7                      # Last 7 days (--days 7)
agy-usage --since 2026-11-01        # Custom start/end date (--until YYYY-MM-DD)
agy-usage -s cli                    # Surface filter: all | ide | cli
agy-usage -p my-project             # Filter by workspace/project or session title
agy-usage -m neon                   # Filter by model name

# Display & export options
agy-usage --sort recent --top 20    # Sort sessions by: tokens | recent | steps
agy-usage --flat                    # Expand subagent sessions instead of parent rollup
agy-usage --raw                     # Show exact comma-grouped integers instead of K/M/B
agy-usage --lang en                 # Switch UI language: auto | en | zh
agy-usage --json                    # Export structured JSON report
```

---

## Data Locations & Environment Variables

By default, `agy-usage` reads local Antigravity data from:

| Surface | Default Path (Linux / macOS) | Default Path (Windows) | Env Override | Flag Override |
| :--- | :--- | :--- | :--- | :--- |
| **Antigravity IDE / App** | `~/.gemini/antigravity` | `%USERPROFILE%\.gemini\antigravity` | `AGY_DATA_DIR` | `--agy-dir` |
| **Antigravity CLI (`agy`)** | `~/.gemini/antigravity-cli` | `%USERPROFILE%\.gemini\antigravity-cli` | `AGY_CLI_DATA_DIR` | `--cli-dir` |

Other supported environment variables:
- `AGY_LANG=zh|en`: Set default dashboard language.
- `NO_COLOR=1`: Disable ANSI terminal colors.

---

## License

[MIT](./LICENSE) © 2026 LiangNing7
