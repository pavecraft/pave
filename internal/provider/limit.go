package provider

import (
	"regexp"
	"strings"
	"time"
)

// limitPhrases are case-insensitive substrings that signal a usage/rate limit in
// provider output.
var limitPhrases = []string{
	"rate limit",
	"rate_limit",
	"usage limit",
	"usage_limit",
	"429",
	"too many requests",
	"quota exceeded",
	// Claude CLI specific messages
	"session limit",
	"claude.ai usage limit",
	"you have reached your",
	"limit reached",
	"plan's usage limit",
	"exceeded your",
}

// resetPattern matches Claude's "resets 2pm (Asia/Saigon)" format.
// Groups: 1=time (e.g. "2pm"), 2=timezone (e.g. "Asia/Saigon").
var resetPattern = regexp.MustCompile(`(?i)resets\s+(\d{1,2}(?::\d{2})?\s*(?:am|pm))\s+\(([^)]+)\)`)

// DetectLimit reports whether a failed result indicates a rate/usage limit.
// It also attempts to parse an explicit reset time from the output.
// The second return value is a human-readable reason/reset description.
// The third return value is the parsed reset time, or zero if not found.
func DetectLimit(res Result) (bool, string) {
	if res.Success {
		return false, ""
	}
	hay := strings.ToLower(res.Stderr + "\n" + res.Output)
	for _, p := range limitPhrases {
		if strings.Contains(hay, p) {
			return true, p
		}
	}
	return false, ""
}

// ParseResetTime tries to extract an explicit reset time from provider output,
// e.g. "resets 2pm (Asia/Saigon)". Returns zero time if not found or unparseable.
func ParseResetTime(output string) time.Time {
	m := resetPattern.FindStringSubmatch(output)
	if m == nil {
		return time.Time{}
	}
	timeStr, tzName := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}
	}

	// Try parsing with minutes first, then hour-only.
	now := time.Now().In(loc)
	for _, layout := range []string{"3:04pm", "3pm"} {
		t, err := time.ParseInLocation(layout, strings.ToLower(timeStr), loc)
		if err != nil {
			continue
		}
		// Build a candidate reset time on today's date.
		reset := time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, loc)
		// If reset is in the past, it must be tomorrow.
		if reset.Before(now) {
			reset = reset.Add(24 * time.Hour)
		}
		return reset
	}
	return time.Time{}
}
