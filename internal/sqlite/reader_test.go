package sqlite

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTableWithOverflowSplitAndWAL(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available to generate fixture sqlite db")
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	// Use Python's standard sqlite3 module to create a database with:
	// - Small page size (1024) to force interior B-tree page splits and overflow pages
	// - WAL mode active with uncommitted/committed frames uncheckpointed in test.db-wal
	script := `
import sqlite3, sys
db = sys.argv[1]
conn = sqlite3.connect(db)
conn.execute("PRAGMA page_size = 1024")
conn.execute("PRAGMA journal_mode = WAL")
conn.execute("PRAGMA wal_autocheckpoint = 0")
conn.execute("""
CREATE TABLE conversation_summaries (
    conversation_id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT "",
    step_count INTEGER NOT NULL DEFAULT 0,
    workspace_uris TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT "",
    parent_conversation_id TEXT NOT NULL DEFAULT "",
    nesting_depth INTEGER NOT NULL DEFAULT 0,
    last_modified_time TEXT NOT NULL,
    raw_summary BLOB
)
""")
big_blob = b"X" * 3500  # Exceeds 1024 page size -> forces multi-page overflow chain!
for i in range(60):     # 60 rows -> forces multi-level interior B-tree split!
    conn.execute(
        "INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        (f"cid-{i:03d}", f"Title {i}", i * 10, '["file:///workspace/demo"]',
         "CORTEX_TRAJECTORY_SOURCE_CLI" if i % 2 == 0 else "", "", 0,
         "2026-11-25T08:00:00Z", big_blob if i == 7 else b"short")
    )
conn.commit()
# Update one row in a second WAL transaction without checkpointing!
conn.execute("UPDATE conversation_summaries SET title = ?, step_count = ? WHERE conversation_id = ?",
             ("Updated In WAL", 999, "cid-007"))
conn.commit()
`
	out, err := exec.Command(py, "-c", script, dbPath).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to generate sqlite fixture: %v (%s)", err, string(out))
	}

	rows, err := ReadTable(dbPath, "conversation_summaries")
	if err != nil {
		t.Fatalf("ReadTable error: %v", err)
	}
	if len(rows) != 60 {
		t.Fatalf("expected 60 rows, got %d", len(rows))
	}

	byID := make(map[string]Row, len(rows))
	for _, r := range rows {
		byID[r.String("conversation_id")] = r
	}

	r7, ok := byID["cid-007"]
	if !ok {
		t.Fatalf("missing cid-007")
	}
	if got := r7.String("title"); got != "Updated In WAL" {
		t.Errorf("expected WAL-updated title 'Updated In WAL', got %q", got)
	}
	if got := r7.Int64("step_count"); got != 999 {
		t.Errorf("expected WAL-updated step_count 999, got %d", got)
	}
	blob, ok := r7["raw_summary"].([]byte)
	if !ok || len(blob) != 3500 || blob[0] != 'X' || blob[3499] != 'X' {
		t.Errorf("overflow blob corrupted: len=%d", len(blob))
	}

	r8 := byID["cid-008"]
	if !strings.Contains(r8.String("source"), "CLI") {
		t.Errorf("expected CLI source for cid-008, got %q", r8.String("source"))
	}
}
