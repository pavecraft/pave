// Package project owns the Feature domain model and the parser that reads a
// feature spec file into a typed list. It performs no I/O beyond reading the
// spec file content passed to it.
package project

import (
	"regexp"
	"strings"
)

// Status is the implementation state of a feature.
type Status string

const (
	// StatusPending means the feature has not been started.
	StatusPending Status = "pending"
	// StatusInProgress means a provider is currently working on the feature.
	StatusInProgress Status = "in_progress"
	// StatusImplemented means the feature has been completed successfully.
	StatusImplemented Status = "implemented"
	// StatusFailed means the last attempt to implement the feature failed.
	StatusFailed Status = "failed"
)

// Valid reports whether s is a recognized Status value.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusImplemented, StatusFailed:
		return true
	default:
		return false
	}
}

// Feature is a single planned unit of work from the spec.
type Feature struct {
	ID          string   // stable slug derived from the title
	Title       string   // human-readable title
	Description string   // optional longer description
	Status      Status   // current implementation state
	DependsOn   []string // IDs of prerequisite features
	Priority    int      // lower = higher priority
}

var (
	nonAlphaNum  = regexp.MustCompile(`[^a-z0-9]+`)
	trimDashes   = regexp.MustCompile(`^-+|-+$`)
	collapseDash = regexp.MustCompile(`-{2,}`)
)

// Slug converts a title into a stable, URL- and filesystem-safe identifier.
// It lowercases, replaces runs of non-alphanumeric characters with a single
// dash, and trims leading/trailing dashes. Empty or symbol-only input yields
// the empty string.
func Slug(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = collapseDash.ReplaceAllString(s, "-")
	s = trimDashes.ReplaceAllString(s, "")
	return s
}
