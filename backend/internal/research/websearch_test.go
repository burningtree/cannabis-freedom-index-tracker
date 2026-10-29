package research

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// A headless run chooses a web search provider up front, so OpenCode never has to ask (and cancel).
func TestEnableWebSearchChoosesAProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE `kv` (`key` text PRIMARY KEY, `value` text NOT NULL, `time_created` integer NOT NULL, `time_updated` integer NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	enableWebSearch(path, "random")
	enableWebSearch(path, "exa") // a second time sets the new choice
	var value string
	if err = db.QueryRow("SELECT value FROM kv WHERE key='websearch:provider'").Scan(&value); err != nil || value != `"exa"` {
		t.Fatalf("websearch provider = %q, %v", value, err)
	}
	enableWebSearch(filepath.Join(t.TempDir(), "missing.db"), "exa") // no database: nothing happens
}
