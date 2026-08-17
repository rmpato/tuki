// Package store keeps tuki's tasks in a single local JSON file. No server, no
// database, no network — just one small file you can read, back up, or delete.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/rmpato/tuki/internal/task"
)

// Version is the on-disk schema version. It exists so a future tuki can
// migrate a file written by this one.
const Version = 1

// Settings are the few things tuki lets you turn off. There is deliberately
// nothing here about streaks, points, or productivity scores.
type Settings struct {
	// Celebrate shows a tiny reaction when you complete something.
	Celebrate bool `json:"celebrate"`
	// Judge lets tuki comment when a lot of things are overdue.
	Judge bool `json:"judge"`
}

// DefaultSettings returns tuki with its personality switched on.
func DefaultSettings() Settings {
	return Settings{Celebrate: true, Judge: true}
}

// Data is the whole contents of the tuki file.
type Data struct {
	Version  int         `json:"version"`
	NextID   int         `json:"next_id"`
	Settings Settings    `json:"settings"`
	Tasks    []task.Task `json:"tasks"`
}

// Store reads and writes a Data at a fixed path.
type Store struct {
	path string
}

// New returns a Store backed by an explicit file path.
func New(path string) *Store { return &Store{path: path} }

// Default returns a Store at tuki's usual home, honouring TUKI_FILE,
// TUKI_HOME, and XDG_DATA_HOME in that order.
func Default() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return New(path), nil
}

// DefaultPath resolves where tuki keeps its file without touching the disk.
func DefaultPath() (string, error) {
	if p := os.Getenv("TUKI_FILE"); p != "" {
		return p, nil
	}
	if dir := os.Getenv("TUKI_HOME"); dir != "" {
		return filepath.Join(dir, "tasks.json"), nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "tuki", "tasks.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("tuki can't find your home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "tuki", "tasks.json"), nil
}

// Path is where this store reads and writes.
func (s *Store) Path() string { return s.path }

// Load reads the tuki file. A missing file is not an error: it just means you
// haven't told tuki anything yet.
func (s *Store) Load() (*Data, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Data{Version: Version, NextID: 1, Settings: DefaultSettings()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.path, err)
	}

	// Settings is decoded through a pointer so a file that predates it — or one
	// written by hand — gets tuki's defaults rather than everything switched off.
	var raw struct {
		Data
		Settings *Settings `json:"settings"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s looks corrupted: %w", s.path, err)
	}

	d := raw.Data
	if raw.Settings != nil {
		d.Settings = *raw.Settings
	} else {
		d.Settings = DefaultSettings()
	}
	d.normalize()
	return &d, nil
}

// normalize repairs anything a hand-edited or older file might be missing.
func (d *Data) normalize() {
	if d.Version == 0 {
		d.Version = Version
	}
	maxID := 0
	for i := range d.Tasks {
		t := &d.Tasks[i]
		t.Tag = task.NormalizeTag(t.Tag)
		if t.CreatedAt.IsZero() {
			t.CreatedAt = time.Now()
		}
		if t.ID > maxID {
			maxID = t.ID
		}
	}
	if d.NextID <= maxID {
		d.NextID = maxID + 1
	}
	if d.NextID < 1 {
		d.NextID = 1
	}
}

// Save writes the file atomically, so an interrupted write can never leave you
// with half a task list.
func (s *Store) Save(d *Data) error {
	d.Version = Version
	d.normalize()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(s.path), err)
	}

	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding tasks: %w", err)
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".tuki-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("saving %s: %w", s.path, err)
	}
	return nil
}

// Add appends a new task and returns it with its assigned ID.
func (d *Data) Add(text, tag string, due *time.Time) task.Task {
	t := task.Task{
		ID:        d.NextID,
		Text:      text,
		Tag:       task.NormalizeTag(tag),
		CreatedAt: time.Now(),
		Due:       due,
	}
	d.NextID++
	d.Tasks = append(d.Tasks, t)
	return t
}

// Find returns a pointer to the task with the given ID so callers can mutate
// it in place.
func (d *Data) Find(id int) (*task.Task, bool) {
	for i := range d.Tasks {
		if d.Tasks[i].ID == id {
			return &d.Tasks[i], true
		}
	}
	return nil, false
}

// SetDone marks a task done or undone, keeping DoneAt in sync.
func (d *Data) SetDone(id int, done bool) (task.Task, bool) {
	t, ok := d.Find(id)
	if !ok {
		return task.Task{}, false
	}
	t.Done = done
	if done {
		now := time.Now()
		t.DoneAt = &now
	} else {
		t.DoneAt = nil
	}
	return *t, true
}

// Toggle flips a task's done state and reports the new state.
func (d *Data) Toggle(id int) (task.Task, bool) {
	t, ok := d.Find(id)
	if !ok {
		return task.Task{}, false
	}
	return d.SetDone(id, !t.Done)
}

// Remove deletes a task, returning it and its former position so it can be
// put back exactly where it was.
func (d *Data) Remove(id int) (removed task.Task, index int, ok bool) {
	for i, t := range d.Tasks {
		if t.ID == id {
			d.Tasks = append(d.Tasks[:i], d.Tasks[i+1:]...)
			return t, i, true
		}
	}
	return task.Task{}, 0, false
}

// Insert puts a task back at a given index, which is how undo works.
func (d *Data) Insert(t task.Task, index int) {
	if index < 0 || index > len(d.Tasks) {
		index = len(d.Tasks)
	}
	d.Tasks = append(d.Tasks, task.Task{})
	copy(d.Tasks[index+1:], d.Tasks[index:])
	d.Tasks[index] = t
}

// ClearDone drops every completed task and returns how many went away.
func (d *Data) ClearDone() int {
	kept := d.Tasks[:0]
	var n int
	for _, t := range d.Tasks {
		if t.Done {
			n++
			continue
		}
		kept = append(kept, t)
	}
	d.Tasks = kept
	return n
}
