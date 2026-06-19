package state

import (
	"time"

	"github.com/paveforge/pave/internal/project"
)

// Reconcile merges the parsed feature spec with previously persisted feature
// state. The spec defines which features exist (and their static metadata:
// title, description, priority, dependencies). Persisted state preserves the
// dynamic status across runs.
//
// Rules:
//   - A feature in the spec but not in prior state is new → keep its spec status
//     (pending, or implemented if the author checked it).
//   - A feature in both → take static fields from the spec, but preserve the
//     persisted status, unless the spec explicitly marks it implemented (an
//     author check-off wins, so a human can force-complete a feature).
//   - A feature only in prior state (removed from the spec) is dropped.
//
// The returned rows carry the given runID and a fresh UpdatedAt timestamp.
func Reconcile(spec []project.Feature, prior []FeatureRow, runID string, now time.Time) []FeatureRow {
	priorByID := make(map[string]FeatureRow, len(prior))
	for _, p := range prior {
		priorByID[p.ID] = p
	}

	out := make([]FeatureRow, 0, len(spec))
	for _, f := range spec {
		row := FeatureRow{
			ID:          f.ID,
			RunID:       runID,
			Title:       f.Title,
			Description: f.Description,
			Status:      f.Status,
			Priority:    f.Priority,
			DependsOn:   f.DependsOn,
			UpdatedAt:   now,
		}

		if p, ok := priorByID[f.ID]; ok {
			// Preserve persisted status unless the spec forces "implemented".
			if f.Status != project.StatusImplemented {
				row.Status = p.Status
			}
		}
		out = append(out, row)
	}
	return out
}
