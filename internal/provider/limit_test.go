package provider

import (
	"testing"
	"time"
)

func TestParseResetTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		output  string
		wantNil bool
	}{
		{"claude session limit message", "You've hit your session limit · resets 2pm (Asia/Saigon)", false},
		{"with minutes", "resets 2:30pm (America/New_York)", false},
		{"uppercase AM", "resets 9AM (UTC)", false},
		{"no reset info", "some unrelated error", true},
		{"malformed timezone", "resets 2pm (Not/AZone)", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseResetTime(tt.output)
			if tt.wantNil && !got.IsZero() {
				t.Errorf("ParseResetTime(%q) = %v, want zero", tt.output, got)
			}
			if !tt.wantNil && got.IsZero() {
				t.Errorf("ParseResetTime(%q) = zero, want non-zero", tt.output)
			}
			// parsed time should be in the future (or at most a second past due to test timing)
			if !got.IsZero() && got.Before(time.Now().Add(-time.Second)) {
				t.Errorf("ParseResetTime(%q) = %v is in the past", tt.output, got)
			}
		})
	}
}

func TestDetectLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		res  Result
		want bool
	}{
		{"success ignored", Result{Success: true, Stderr: "rate limit"}, false},
		{"rate limit stderr", Result{Success: false, Stderr: "Error: rate limit exceeded"}, true},
		{"429 in output", Result{Success: false, Output: "HTTP 429"}, true},
		{"usage limit", Result{Success: false, Stderr: "usage limit reached"}, true},
		{"too many requests", Result{Success: false, Stderr: "Too Many Requests"}, true},
		{"quota", Result{Success: false, Output: "quota exceeded for today"}, true},
		{"claude session limit", Result{Success: false, Output: "You've hit your session limit · resets 2pm (Asia/Saigon)"}, true},
		{"unrelated failure", Result{Success: false, Stderr: "syntax error"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, _ := DetectLimit(tt.res)
			if got != tt.want {
				t.Errorf("DetectLimit(%+v) = %v, want %v", tt.res, got, tt.want)
			}
		})
	}
}
