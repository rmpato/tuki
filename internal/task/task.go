// Package task holds tuki's domain model: a task, its tag, and its optional
// due date. It has no dependencies beyond the standard library so it stays
// cheap to load and easy to test.
package task

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The four tags tuki knows about by name. Any other tag works too; these just
// get a reserved spot and a reserved color.
const (
	TagWork  = "work"
	TagHome  = "home"
	TagHobby = "hobby"
	TagMisc  = "misc"
)

// Known lists the built-in tags in the order they should appear on screen.
var Known = []string{TagWork, TagHome, TagHobby, TagMisc}

// Task is one thing you need to remember to do.
type Task struct {
	ID        int        `json:"id"`
	Text      string     `json:"text"`
	Tag       string     `json:"tag,omitempty"`
	Done      bool       `json:"done"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
	Due       *time.Time `json:"due,omitempty"`
}

// Overdue reports whether the task has a due date that has already passed.
// Comparison is by calendar day, not by instant: something due today is not
// overdue at 11pm.
func (t Task) Overdue(now time.Time) bool {
	if t.Done || t.Due == nil {
		return false
	}
	return StartOfDay(*t.Due).Before(StartOfDay(now))
}

// DueToday reports whether the task is due on the current calendar day.
func (t Task) DueToday(now time.Time) bool {
	if t.Done || t.Due == nil {
		return false
	}
	return StartOfDay(*t.Due).Equal(StartOfDay(now))
}

// StartOfDay truncates a time to local midnight.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// NormalizeTag lowercases a tag, drops a leading '#', and strips anything that
// isn't a letter, digit, dash, or underscore. An empty result becomes "misc",
// so every task always belongs to exactly one group.
func NormalizeTag(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "#")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return -1
	}, s)
	if s == "" {
		return TagMisc
	}
	return s
}

// SortTags orders tags for display: work, home, hobby, then any custom tags
// alphabetically, and misc last because it's the catch-all.
func SortTags(tags []string) []string {
	rank := func(t string) int {
		switch t {
		case TagWork:
			return 0
		case TagHome:
			return 1
		case TagHobby:
			return 2
		case TagMisc:
			return 4
		default:
			return 3
		}
	}
	out := append([]string(nil), tags...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rank(out[i]), rank(out[j])
		if ri != rj {
			return ri < rj
		}
		return out[i] < out[j]
	})
	return out
}

// Group buckets tasks by tag and returns the tags in display order.
func Group(tasks []Task) ([]string, map[string][]Task) {
	byTag := make(map[string][]Task)
	for _, t := range tasks {
		tag := NormalizeTag(t.Tag)
		byTag[tag] = append(byTag[tag], t)
	}
	tags := make([]string, 0, len(byTag))
	for tag := range byTag {
		tags = append(tags, tag)
	}
	return SortTags(tags), byTag
}

// Stats counts how many of the given tasks are done.
func Stats(tasks []Task) (done, total int) {
	for _, t := range tasks {
		if t.Done {
			done++
		}
	}
	return done, len(tasks)
}

// CountOverdue returns how many of the given tasks are past due.
func CountOverdue(tasks []Task, now time.Time) int {
	var n int
	for _, t := range tasks {
		if t.Overdue(now) {
			n++
		}
	}
	return n
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "weds": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

var relativeRe = regexp.MustCompile(`^\+?(\d+)\s*([dwm])$`)

// ErrNoDue is returned by ParseDue for input that clears the due date.
var ErrNoDue = fmt.Errorf("no due date")

// ParseDue turns a friendly string into a due date pinned to local midnight.
//
// It understands "today", "tomorrow", "yesterday", weekday names ("fri"),
// relative offsets ("+3d", "2w", "1m"), and dates ("2026-08-20", "08-20").
// The empty string, "none", "-", and "clear" return a nil time, which means
// "this task has no due date".
func ParseDue(s string, now time.Time) (*time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "@")
	today := StartOfDay(now)

	switch s {
	case "", "none", "-", "clear", "never":
		return nil, nil
	case "today", "tod", "now":
		return &today, nil
	case "tomorrow", "tom", "tmr", "tmw":
		d := today.AddDate(0, 0, 1)
		return &d, nil
	case "yesterday", "yest":
		d := today.AddDate(0, 0, -1)
		return &d, nil
	}

	if s == "next week" || s == "nextweek" {
		d := today.AddDate(0, 0, 7)
		return &d, nil
	}

	if wd, ok := weekdays[s]; ok {
		// The named weekday on or after today, so "fri" on a Friday means today.
		diff := (int(wd) - int(today.Weekday()) + 7) % 7
		d := today.AddDate(0, 0, diff)
		return &d, nil
	}

	if m := relativeRe.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("bad offset %q", s)
		}
		var d time.Time
		switch m[2] {
		case "d":
			d = today.AddDate(0, 0, n)
		case "w":
			d = today.AddDate(0, 0, 7*n)
		case "m":
			d = today.AddDate(0, n, 0)
		}
		return &d, nil
	}

	if d, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return &d, nil
	}
	if d, err := time.ParseInLocation("01-02", s, now.Location()); err == nil {
		d = time.Date(today.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		if d.Before(today) {
			d = d.AddDate(1, 0, 0) // "01-05" in December means next January
		}
		return &d, nil
	}

	return nil, fmt.Errorf("tuki doesn't know when %q is", s)
}

// HumanDue renders a due date the way a friend would say it out loud.
func HumanDue(due, now time.Time) string {
	d := StartOfDay(due)
	today := StartOfDay(now)
	days := int(d.Sub(today).Hours() / 24)

	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days == -1:
		return "yesterday"
	case days > 1 && days < 7:
		return strings.ToLower(d.Weekday().String()[:3])
	case days < -1 && days > -7:
		return fmt.Sprintf("%dd ago", -days)
	}
	if d.Year() != today.Year() {
		return strings.ToLower(d.Format("2 Jan 2006"))
	}
	return strings.ToLower(d.Format("2 Jan"))
}

var (
	tagTokenRe = regexp.MustCompile(`(^|\s)#([^\s]+)`)
	dueTokenRe = regexp.MustCompile(`(^|\s)@([^\s]+)`)
)

// Inline is the result of pulling "#tag" and "@due" out of a line of text.
type Inline struct {
	Text string
	Tag  string
	Due  *time.Time
}

// ParseInline extracts a trailing-or-inline "#tag" and "@when" from free text
// so `tuki add "email sam #work @fri"` does the obvious thing.
//
// A token that doesn't parse as a date is deliberately left in the text rather
// than reported as an error, so "email @sam" stays a task about emailing sam.
func ParseInline(s string, now time.Time) Inline {
	in := Inline{}

	if m := tagTokenRe.FindStringSubmatch(s); m != nil {
		if tag := NormalizeTag(m[2]); tag != "" {
			in.Tag = tag
			s = strings.Replace(s, m[0], m[1], 1)
		}
	}
	if m := dueTokenRe.FindStringSubmatch(s); m != nil {
		if due, err := ParseDue(m[2], now); err == nil && due != nil {
			in.Due = due
			s = strings.Replace(s, m[0], m[1], 1)
		}
	}

	in.Text = strings.Join(strings.Fields(s), " ")
	return in
}

// FormatInline is the inverse of ParseInline: it rebuilds an editable line
// from a task, so editing in the TUI shows "#tag" and "@due" back to you.
func FormatInline(t Task, now time.Time) string {
	var b strings.Builder
	b.WriteString(t.Text)
	if tag := NormalizeTag(t.Tag); tag != TagMisc {
		fmt.Fprintf(&b, " #%s", tag)
	}
	if t.Due != nil {
		fmt.Fprintf(&b, " @%s", t.Due.Format("2006-01-02"))
	}
	return b.String()
}
