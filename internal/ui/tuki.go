package ui

import (
	"fmt"
	"math/rand"
	"strings"
)

// tuki is a small creature of indeterminate species. These are its faces.
const (
	FaceIdle    = "(o.o)" // just hanging out
	FaceHappy   = "(^.^)" // you finished something
	FaceCheer   = "(^o^)" // you finished everything
	FaceSide    = "(-.-)" // a lot of things are late
	FaceSleepy  = "(u.u)" // there is nothing to do
	FaceCurious = "(o.O)" // you're typing at it
)

// Blob is tuki at full size, for when there's room to be sentimental.
func Blob(face string) string {
	eyes := strings.TrimSuffix(strings.TrimPrefix(face, "("), ")")
	return strings.Join([]string{
		`  .--.  `,
		fmt.Sprintf(` ( %s ) `, eyes),
		`  '--'  `,
	}, "\n")
}

var cheers = []string{
	"nice",
	"done!",
	"tuki approves",
	"one less thing",
	"good one",
	"gone",
	"that's the stuff",
	"tidy",
	"less to carry",
	"noted, with respect",
}

// Cheer is what tuki says when you complete something.
func Cheer() string { return cheers[rand.Intn(len(cheers))] }

var allDone = []string{
	"everything's done. tuki is a little suspicious.",
	"all clear. go outside.",
	"nothing left. tuki is impressed.",
	"done, done, and done.",
}

// AllDone is what tuki says when the list is finished.
func AllDone() string { return allDone[rand.Intn(len(allDone))] }

var empty = []string{
	"nothing here yet.",
	"an empty list. bold.",
	"no tasks. tuki naps.",
	"blank slate.",
}

// Empty is what tuki says when there are no tasks at all.
func Empty() string { return empty[rand.Intn(len(empty))] }

// Judge is tuki's gentle commentary on overdue tasks. It says nothing at all
// until things get a bit silly, and it never nags twice as hard.
func Judge(overdue int) string {
	switch {
	case overdue < 3:
		return ""
	case overdue < 6:
		return fmt.Sprintf("%d things are late. no pressure.", overdue)
	case overdue < 11:
		return fmt.Sprintf("%d overdue. tuki is being polite about it.", overdue)
	case overdue < 21:
		return fmt.Sprintf("%d overdue. tuki has noticed. tuki says nothing.", overdue)
	case overdue < 48:
		return fmt.Sprintf("%d overdue. we're past judgement now. we're just here.", overdue)
	default:
		return fmt.Sprintf("%d overdue. tuki respects the commitment to the bit.", overdue)
	}
}

// Greeting is the small line under an empty-ish list.
func Greeting(total int) string {
	if total == 0 {
		return Empty()
	}
	return ""
}
