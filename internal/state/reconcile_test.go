package state

import (
	"testing"
	"time"

	"github.com/pavecraft/pave/internal/project"
)

func TestReconcile(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name       string
		spec       []project.Feature
		prior      []FeatureRow
		wantStatus map[string]project.Status
		wantIDs    []string
	}{
		{
			name: "new feature keeps spec status",
			spec: []project.Feature{
				{ID: "f1", Title: "One", Status: project.StatusPending},
			},
			prior:      nil,
			wantStatus: map[string]project.Status{"f1": project.StatusPending},
			wantIDs:    []string{"f1"},
		},
		{
			name: "persisted status preserved over spec pending",
			spec: []project.Feature{
				{ID: "f1", Title: "One", Status: project.StatusPending},
			},
			prior: []FeatureRow{
				{ID: "f1", Status: project.StatusImplemented},
			},
			wantStatus: map[string]project.Status{"f1": project.StatusImplemented},
			wantIDs:    []string{"f1"},
		},
		{
			name: "spec check-off forces implemented",
			spec: []project.Feature{
				{ID: "f1", Title: "One", Status: project.StatusImplemented},
			},
			prior: []FeatureRow{
				{ID: "f1", Status: project.StatusPending},
			},
			wantStatus: map[string]project.Status{"f1": project.StatusImplemented},
			wantIDs:    []string{"f1"},
		},
		{
			name: "in-progress persisted status preserved",
			spec: []project.Feature{
				{ID: "f1", Title: "One", Status: project.StatusPending},
			},
			prior: []FeatureRow{
				{ID: "f1", Status: project.StatusInProgress},
			},
			wantStatus: map[string]project.Status{"f1": project.StatusInProgress},
			wantIDs:    []string{"f1"},
		},
		{
			name: "feature removed from spec is dropped",
			spec: []project.Feature{
				{ID: "f1", Title: "One", Status: project.StatusPending},
			},
			prior: []FeatureRow{
				{ID: "f1", Status: project.StatusPending},
				{ID: "f2", Status: project.StatusImplemented},
			},
			wantStatus: map[string]project.Status{"f1": project.StatusPending},
			wantIDs:    []string{"f1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Reconcile(tt.spec, tt.prior, "run1", now)

			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d rows, want %d (%v)", len(got), len(tt.wantIDs), got)
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("row[%d].ID = %q, want %q", i, got[i].ID, id)
				}
				if got[i].RunID != "run1" {
					t.Errorf("row[%d].RunID = %q, want run1", i, got[i].RunID)
				}
				if got[i].Status != tt.wantStatus[id] {
					t.Errorf("row[%d] (%s) status = %q, want %q", i, id, got[i].Status, tt.wantStatus[id])
				}
			}
		})
	}
}
