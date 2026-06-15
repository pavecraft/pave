package provider

import (
	"strings"
)

// BuildPrompt constructs the instruction text sent to a coding CLI for a task.
// It is deterministic and includes the feature title, description, any extra
// context, and explicit guardrails so the provider stays focused on a single
// feature.
func BuildPrompt(t Task) string {
	var b strings.Builder

	b.WriteString("You are implementing a single feature in an existing project.\n\n")

	b.WriteString("# Feature\n")
	b.WriteString("Title: ")
	b.WriteString(t.Feature.Title)
	b.WriteByte('\n')

	if id := strings.TrimSpace(t.Feature.ID); id != "" {
		b.WriteString("ID: ")
		b.WriteString(id)
		b.WriteByte('\n')
	}

	if desc := strings.TrimSpace(t.Feature.Description); desc != "" {
		b.WriteString("\nDescription:\n")
		b.WriteString(desc)
		b.WriteByte('\n')
	}

	if len(t.Feature.DependsOn) > 0 {
		b.WriteString("\nThis feature depends on (assume already implemented): ")
		b.WriteString(strings.Join(t.Feature.DependsOn, ", "))
		b.WriteByte('\n')
	}

	if ctx := strings.TrimSpace(t.Context); ctx != "" {
		b.WriteString("\n# Additional context\n")
		b.WriteString(ctx)
		b.WriteByte('\n')
	}

	b.WriteString("\n# Instructions\n")
	b.WriteString("- Implement ONLY this feature. Do not start other features.\n")
	b.WriteString("- Make the smallest coherent change that fully satisfies it.\n")
	b.WriteString("- Add or update tests for the code you change.\n")
	b.WriteString("- Do not commit, push, or run destructive git commands.\n")

	return b.String()
}
