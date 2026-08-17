package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rmpato/tuki/internal/task"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "tasks.json"))
}

// A first run has no file, and that is not an error.
func TestLoadMissingFileIsEmpty(t *testing.T) {
	s := tempStore(t)

	d, err := s.Load()
	if err != nil {
		t.Fatalf("Load on a missing file: %v", err)
	}
	if len(d.Tasks) != 0 {
		t.Errorf("got %d tasks, want 0", len(d.Tasks))
	}
	if d.NextID != 1 {
		t.Errorf("NextID = %d, want 1", d.NextID)
	}
	if !d.Settings.Celebrate || !d.Settings.Judge {
		t.Error("a fresh store should have tuki's personality switched on")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()

	due := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	d.Add("buy oat milk", "home", &due)
	d.Add("write the docs", "work", nil)

	if err := s.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(got.Tasks))
	}
	if got.Tasks[0].Text != "buy oat milk" || got.Tasks[0].Tag != "home" {
		t.Errorf("first task = %+v", got.Tasks[0])
	}
	if got.Tasks[0].Due == nil || !got.Tasks[0].Due.Equal(due) {
		t.Errorf("due date did not survive: %v", got.Tasks[0].Due)
	}
	if got.NextID != 3 {
		t.Errorf("NextID = %d, want 3", got.NextID)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()

	a := d.Add("one", "", nil)
	d.Add("two", "", nil)
	d.Remove(a.ID)

	c := d.Add("three", "", nil)
	if c.ID == a.ID {
		t.Errorf("new task reused id %d", c.ID)
	}
}

func TestSetDoneTracksTimestamp(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()
	tk := d.Add("ship it", "work", nil)

	done, ok := d.SetDone(tk.ID, true)
	if !ok || !done.Done || done.DoneAt == nil {
		t.Fatalf("SetDone(true) = %+v, ok=%v", done, ok)
	}

	undone, _ := d.SetDone(tk.ID, false)
	if undone.Done || undone.DoneAt != nil {
		t.Errorf("SetDone(false) left DoneAt = %v", undone.DoneAt)
	}
}

// Removing and re-inserting is how undo works, so position has to be exact.
func TestRemoveAndInsertRestoresPosition(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()
	d.Add("first", "", nil)
	middle := d.Add("middle", "", nil)
	d.Add("last", "", nil)

	removed, idx, ok := d.Remove(middle.ID)
	if !ok || idx != 1 {
		t.Fatalf("Remove returned idx=%d ok=%v", idx, ok)
	}
	d.Insert(removed, idx)

	if len(d.Tasks) != 3 || d.Tasks[1].Text != "middle" {
		t.Errorf("after re-insert: %v", texts(d.Tasks))
	}
}

func TestClearDoneKeepsUnfinished(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()
	a := d.Add("done one", "", nil)
	d.Add("still todo", "", nil)
	b := d.Add("done two", "", nil)
	d.SetDone(a.ID, true)
	d.SetDone(b.ID, true)

	if n := d.ClearDone(); n != 2 {
		t.Errorf("ClearDone = %d, want 2", n)
	}
	if len(d.Tasks) != 1 || d.Tasks[0].Text != "still todo" {
		t.Errorf("after clear: %v", texts(d.Tasks))
	}
}

// A file written by hand — no settings, no next_id — should still work.
func TestLoadRepairsSparseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	raw := `{"tasks":[{"id":7,"text":"hand written","tag":"WORK"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := New(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.NextID != 8 {
		t.Errorf("NextID = %d, want 8 (past the highest existing id)", d.NextID)
	}
	if d.Tasks[0].Tag != "work" {
		t.Errorf("tag = %q, want normalized to %q", d.Tasks[0].Tag, "work")
	}
	if d.Tasks[0].CreatedAt.IsZero() {
		t.Error("a missing created_at should have been filled in")
	}
	if !d.Settings.Celebrate {
		t.Error("missing settings should fall back to the defaults")
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "nested", "deeper", "tasks.json"))

	d, _ := s.Load()
	d.Add("make the directories", "", nil)
	if err := s.Save(d); err != nil {
		t.Fatalf("Save into a missing directory: %v", err)
	}

	// The temp file used during the write must not be left behind.
	entries, err := os.ReadDir(filepath.Dir(s.Path()))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "tasks.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want just tasks.json", names)
	}
}

func TestSavedFileIsReadableJSON(t *testing.T) {
	s := tempStore(t)
	d, _ := s.Load()
	d.Add("readable", "home", nil)
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	if err := json.Unmarshal(b, &check); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if b[len(b)-1] != '\n' {
		t.Error("saved file should end with a newline")
	}
}

func TestDefaultPathHonoursEnvironment(t *testing.T) {
	t.Setenv("TUKI_FILE", "")
	t.Setenv("TUKI_HOME", "/tmp/tuki-home")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")

	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("/tmp/tuki-home", "tasks.json") {
		t.Errorf("DefaultPath = %q", got)
	}

	t.Setenv("TUKI_FILE", "/tmp/exact.json")
	if got, _ := DefaultPath(); got != "/tmp/exact.json" {
		t.Errorf("TUKI_FILE should win, got %q", got)
	}
}

func texts(tasks []task.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Text
	}
	return out
}
