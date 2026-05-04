// Package ui provides minimal ANSI styling helpers in GitHub Copilot CLI style.
// It respects NO_COLOR and falls back to plain text when stdout is not a TTY.
package ui

import (
	"fmt"
	"os"
)

var tty = isTTY()

func isTTY() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func apply(codes, s string) string {
	if !tty {
		return s
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", codes, s)
}

// Bold returns s in bold.
func Bold(s string) string { return apply("1", s) }

// Dim returns s dimmed.
func Dim(s string) string { return apply("2", s) }

// Cyan returns s in cyan.
func Cyan(s string) string { return apply("36", s) }

// Green returns s in green (success).
func Green(s string) string { return apply("32", s) }

// Yellow returns s in yellow (warning).
func Yellow(s string) string { return apply("33", s) }

// Gray returns s in gray (secondary info/labels).
func Gray(s string) string { return apply("90", s) }

// Header renders s bold and cyan (used for section headers).
func Header(s string) string { return apply("1;36", s) }

// Sep returns a dim separator line of 60 box-drawing dashes.
func Sep() string {
	const line = "────────────────────────────────────────────────────────────"
	return apply("2", line)
}
