package task

import (
	"testing"
	"time"
)

// A Monday, so weekday arithmetic in the tests is easy to reason about.
var ref = time.Date(2026, time.August, 17, 14, 30, 0, 0, time.UTC)

func TestNormalizeTag(t *testing.T) {
	cases := map[string]string{
		"":          TagMisc,
		"  ":        TagMisc,
		"Work":      TagWork,
		"#home":     TagHome,
		"HOBBY!!":   TagHobby,
		"side-車":    "side-車",
		"a b":       "ab",
		"#!!!":      TagMisc,
		"deep_work": "deep_work",
	}
	for in, want := range cases {
		if got := NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortTagsPutsMiscLast(t *testing.T) {
	got := SortTags([]string{"misc", "zebra", "home", "apple", "work", "hobby"})
	want := []string{"work", "home", "hobby", "apple", "zebra", "misc"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortTags = %v, want %v", got, want)
		}
	}
}

func TestParseDue(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		in   string
		want *time.Time
	}{
		{"", nil},
		{"none", nil},
		{"today", ptr(day(2026, time.August, 17))},
		{"tomorrow", ptr(day(2026, time.August, 18))},
		{"yesterday", ptr(day(2026, time.August, 16))},
		{"mon", ptr(day(2026, time.August, 17))}, // today is Monday
		{"fri", ptr(day(2026, time.August, 21))},
		{"sun", ptr(day(2026, time.August, 23))},
		{"+3d", ptr(day(2026, time.August, 20))},
		{"2w", ptr(day(2026, time.August, 31))},
		{"1m", ptr(day(2026, time.September, 17))},
		{"2026-12-01", ptr(day(2026, time.December, 1))},
		{"@fri", ptr(day(2026, time.August, 21))},
	}

	for _, c := range cases {
		got, err := ParseDue(c.in, ref)
		if err != nil {
			t.Errorf("ParseDue(%q) errored: %v", c.in, err)
			continue
		}
		switch {
		case c.want == nil && got != nil:
			t.Errorf("ParseDue(%q) = %v, want nil", c.in, got)
		case c.want != nil && got == nil:
			t.Errorf("ParseDue(%q) = nil, want %v", c.in, *c.want)
		case c.want != nil && !got.Equal(*c.want):
			t.Errorf("ParseDue(%q) = %v, want %v", c.in, *got, *c.want)
		}
	}

	if _, err := ParseDue("whenever", ref); err == nil {
		t.Error("ParseDue(\"whenever\") should have failed")
	}
}

// A bare month-day that has already passed this year means next year.
func TestParseDueRollsMonthDayForward(t *testing.T) {
	got, err := ParseDue("01-05", ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2027 {
		t.Errorf("ParseDue(\"01-05\") = %v, want a 2027 date", *got)
	}
}

func TestOverdueIsByCalendarDay(t *testing.T) {
	// Due this morning, checked this afternoon: still not overdue.
	dueToday := StartOfDay(ref)
	tk := Task{Due: &dueToday}
	if tk.Overdue(ref) {
		t.Error("a task due today should not be overdue")
	}
	if !tk.DueToday(ref) {
		t.Error("a task due today should report DueToday")
	}

	yesterday := dueToday.AddDate(0, 0, -1)
	tk.Due = &yesterday
	if !tk.Overdue(ref) {
		t.Error("a task due yesterday should be overdue")
	}

	// Completed tasks are never overdue, however late they were.
	tk.Done = true
	if tk.Overdue(ref) {
		t.Error("a completed task should never be overdue")
	}
}

func TestHumanDue(t *testing.T) {
	cases := map[int]string{
		0:  "today",
		1:  "tomorrow",
		2:  "wednesday"[:3],
		5:  "saturday"[:3],
		-1: "yesterday",
		-3: "3d ago",
	}
	for offset, want := range cases {
		d := StartOfDay(ref).AddDate(0, 0, offset)
		if got := HumanDue(d, ref); got != want {
			t.Errorf("HumanDue(%+dd) = %q, want %q", offset, got, want)
		}
	}

	far := StartOfDay(ref).AddDate(0, 0, 40)
	if got := HumanDue(far, ref); got != "26 sep" {
		t.Errorf("HumanDue(+40d) = %q, want %q", got, "26 sep")
	}
}

func TestParseInline(t *testing.T) {
	in := ParseInline("write the docs #work @fri", ref)
	if in.Text != "write the docs" {
		t.Errorf("Text = %q", in.Text)
	}
	if in.Tag != TagWork {
		t.Errorf("Tag = %q", in.Tag)
	}
	if in.Due == nil || in.Due.Day() != 21 {
		t.Errorf("Due = %v, want 21 Aug", in.Due)
	}
}

// An @token that isn't a date is part of the task, not a broken due date.
func TestParseInlineLeavesNonDates(t *testing.T) {
	in := ParseInline("email @sam about the thing", ref)
	if in.Text != "email @sam about the thing" {
		t.Errorf("Text = %q, want the @sam left alone", in.Text)
	}
	if in.Due != nil {
		t.Errorf("Due = %v, want nil", in.Due)
	}
}

func TestFormatInlineRoundTrips(t *testing.T) {
	due := StartOfDay(ref).AddDate(0, 0, 4)
	original := Task{Text: "learn a chord", Tag: TagHobby, Due: &due}

	back := ParseInline(FormatInline(original, ref), ref)
	if back.Text != original.Text {
		t.Errorf("Text = %q, want %q", back.Text, original.Text)
	}
	if back.Tag != original.Tag {
		t.Errorf("Tag = %q, want %q", back.Tag, original.Tag)
	}
	if back.Due == nil || !back.Due.Equal(due) {
		t.Errorf("Due = %v, want %v", back.Due, due)
	}
}

// misc is the default, so it isn't written back out as "#misc".
func TestFormatInlineOmitsMisc(t *testing.T) {
	got := FormatInline(Task{Text: "call the dentist", Tag: TagMisc}, ref)
	if got != "call the dentist" {
		t.Errorf("FormatInline = %q", got)
	}
}

func TestGroupAndStats(t *testing.T) {
	tasks := []Task{
		{ID: 1, Tag: "work", Done: true},
		{ID: 2, Tag: "work"},
		{ID: 3, Tag: ""},
		{ID: 4, Tag: "home"},
	}

	tags, byTag := Group(tasks)
	want := []string{"work", "home", "misc"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", tags, want)
		}
	}
	if len(byTag["work"]) != 2 {
		t.Errorf("work group has %d tasks, want 2", len(byTag["work"]))
	}

	done, total := Stats(tasks)
	if done != 1 || total != 4 {
		t.Errorf("Stats = %d/%d, want 1/4", done, total)
	}
}

func ptr(t time.Time) *time.Time { return &t }
