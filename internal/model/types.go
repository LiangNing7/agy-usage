package model

import "time"

// Surface identifies which Antigravity product surface created a conversation.
type Surface string

const (
	SurfaceAll Surface = "all"
	SurfaceIDE Surface = "ide"
	SurfaceCLI Surface = "cli"
)

// StepUsage represents a single LLM invocation recorded in transcript.jsonl.
type StepUsage struct {
	ConvID        string
	Surface       Surface
	Project       string
	IsSubagent    bool
	Timestamp     time.Time
	LocalDate     string
	Model         string
	StepType      string
	UncachedInput int64
	CacheRead     int64
	Output        int64
}

func (s StepUsage) TotalInput() int64  { return s.UncachedInput + s.CacheRead }
func (s StepUsage) TotalTokens() int64 { return s.UncachedInput + s.CacheRead + s.Output }
func (s StepUsage) NetTokens() int64   { return s.UncachedInput + s.Output }

// ConvSummaryMeta stores metadata extracted from conversation_summaries.db.
type ConvSummaryMeta struct {
	ConvID       string
	Title        string
	Surface      Surface
	Workspace    string
	Project      string
	ParentID     string
	NestingDepth int
	SummarySteps int
	LastModified time.Time
}

// ConvStats holds token usage metrics for a conversation (and optional subagent rollup).
type ConvStats struct {
	ConvID        string
	Title         string
	Surface       Surface
	Project       string
	Workspace     string
	ParentID      string
	NestingDepth  int
	Steps         int
	Compactions   int
	UncachedInput int64
	CacheRead     int64
	Output        int64
	FirstTS       time.Time
	LastTS        time.Time
	Models        map[string]struct{}

	// Rollup fields (self + descendant subagents)
	SubagentCount       int
	RollupSteps         int
	RollupUncachedInput int64
	RollupCacheRead     int64
	RollupOutput        int64
	RollupLastTS        time.Time
}

func (c *ConvStats) IsSubagent() bool   { return c.ParentID != "" || c.NestingDepth > 0 }
func (c *ConvStats) TotalTokens() int64 { return c.UncachedInput + c.CacheRead + c.Output }
func (c *ConvStats) NetTokens() int64   { return c.UncachedInput + c.Output }
func (c *ConvStats) RollupTotalTokens() int64 {
	return c.RollupUncachedInput + c.RollupCacheRead + c.RollupOutput
}
func (c *ConvStats) RollupNetTokens() int64 { return c.RollupUncachedInput + c.RollupOutput }

// FilterOptions defines user query and display filters.
type FilterOptions struct {
	Today   bool
	Days    int
	Since   string // YYYY-MM-DD
	Until   string // YYYY-MM-DD
	Surface Surface
	Project string
	Model   string
	Flat    bool
	SortBy  string // "tokens" | "recent" | "steps"
	TopN    int
}

// BucketMetrics represents aggregated token metrics for a date, model, project, or surface.
type BucketMetrics struct {
	Key       string `json:"key"`
	Convs     int    `json:"conversations,omitempty"`
	MainConvs int    `json:"main_conversations,omitempty"`
	SubConvs  int    `json:"subagent_conversations,omitempty"`
	Steps     int    `json:"steps"`
	Uncached  int64  `json:"uncached_input"`
	Cache     int64  `json:"cache_read"`
	Output    int64  `json:"output"`
	Net       int64  `json:"net_tokens"`
	Total     int64  `json:"total_tokens"`
}

// TotalsSummary holds top-level KPI metrics.
type TotalsSummary struct {
	TotalTokens           int64   `json:"total_tokens"`
	NetTokens             int64   `json:"net_tokens"`
	TotalInput            int64   `json:"total_input"`
	UncachedInput         int64   `json:"uncached_input"`
	CacheRead             int64   `json:"cache_read"`
	Output                int64   `json:"output"`
	CacheHitRate          float64 `json:"cache_hit_rate"`
	Steps                 int     `json:"steps"`
	Compactions           int     `json:"compactions"`
	Conversations         int     `json:"conversations"`
	MainConversations     int     `json:"main_conversations"`
	SubagentConversations int     `json:"subagent_conversations"`
	SubagentTokens        int64   `json:"subagent_tokens"`
	SubagentSteps         int     `json:"subagent_steps"`
	ActiveDays            int     `json:"active_days"`
}

// SessionView is a serializable view of a conversation row.
type SessionView struct {
	ConversationID string   `json:"conversation_id"`
	Title          string   `json:"title"`
	Surface        Surface  `json:"surface"`
	Project        string   `json:"project"`
	Workspace      string   `json:"workspace"`
	IsSubagent     bool     `json:"is_subagent"`
	SubagentCount  int      `json:"subagent_count"`
	Steps          int      `json:"steps"`
	OwnSteps       int      `json:"own_steps"`
	UncachedInput  int64    `json:"uncached_input"`
	CacheRead      int64    `json:"cache_read"`
	Output         int64    `json:"output"`
	NetTokens      int64    `json:"net_tokens"`
	TotalTokens    int64    `json:"total_tokens"`
	Models         []string `json:"models"`
	LastActive     string   `json:"last_active,omitempty"`
	LastActiveUnix int64    `json:"-"`
}

// FilterEcho echoes applied filters in JSON/UI responses.
type FilterEcho struct {
	Since   string  `json:"since,omitempty"`
	Until   string  `json:"until,omitempty"`
	Surface Surface `json:"surface"`
	Project string  `json:"project,omitempty"`
	Model   string  `json:"model,omitempty"`
}

// Report is the complete aggregated dashboard payload.
type Report struct {
	GeneratedAt               string                    `json:"generated_at"`
	Today                     string                    `json:"today"`
	Filters                   FilterEcho                `json:"filters"`
	Totals                    TotalsSummary             `json:"totals"`
	BySurface                 map[Surface]BucketMetrics `json:"by_surface"`
	ByDate                    []BucketMetrics           `json:"by_date"`
	ByModel                   []BucketMetrics           `json:"by_model"`
	ByProject                 []BucketMetrics           `json:"by_project"`
	Sessions                  []SessionView             `json:"sessions"`
	InaccessibleConversations int                       `json:"inaccessible_conversations"`
	InaccessibleSteps         int                       `json:"inaccessible_steps"`
}
