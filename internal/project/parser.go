package project

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// checkboxLine matches a Markdown task-list item, capturing the check mark and
// the remaining text. Example: "- [x] Config loader".
var checkboxLine = regexp.MustCompile(`^\s*[-*]\s*\[([ xX])\]\s*(.+?)\s*$`)

// metaBlock matches a trailing "(key: value, ...)" metadata block.
var metaBlock = regexp.MustCompile(`\s*\(([^)]*)\)\s*$`)

// ParseFile reads and parses a Markdown feature spec from path.
func ParseFile(path string) ([]Feature, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening features file %q: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads a Markdown feature spec and returns the parsed features.
//
// Each feature is a task-list item:
//
//   - [ ] Title — optional description (priority: 2, depends: f01, f02)
//
// A checked box ([x]) yields StatusImplemented; an unchecked box yields
// StatusPending. The description may follow an em dash ("—"), an en dash ("–"),
// or " - ". A trailing "(...)" block carries metadata: "priority: N" and
// "depends: id1, id2". The feature ID is the slug of the title; if the slug is
// empty the line is skipped.
func Parse(r io.Reader) ([]Feature, error) {
	var features []Feature
	seen := make(map[string]int) // id -> count, for de-duplication suffixes
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		m := checkboxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		status := StatusPending
		if mark := m[1]; mark == "x" || mark == "X" {
			status = StatusImplemented
		}
		feat := parseItem(m[2], status)
		if feat.ID == "" {
			continue
		}
		// Ensure unique IDs by appending -2, -3, ... on collision.
		seen[feat.ID]++
		if n := seen[feat.ID]; n > 1 {
			feat.ID = fmt.Sprintf("%s-%d", feat.ID, n)
		}
		features = append(features, feat)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning features: %w", err)
	}
	return features, nil
}

// parseItem parses the text portion of a checklist item (everything after the
// checkbox) into a Feature with the given status.
func parseItem(text string, status Status) Feature {
	feat := Feature{Status: status}

	// Extract a trailing metadata block, if present.
	if m := metaBlock.FindStringSubmatch(text); m != nil {
		applyMeta(&feat, m[1])
		text = strings.TrimSpace(text[:len(text)-len(m[0])])
	}

	// Split title from description on the first dash separator.
	title, desc := splitTitleDesc(text)
	feat.Title = strings.TrimSpace(title)
	feat.Description = strings.TrimSpace(desc)
	feat.ID = Slug(feat.Title)
	return feat
}

// descSeparators are the separators that divide a title from its description,
// tried in order. Longer/multi-char separators come first.
var descSeparators = []string{" — ", " – ", " -- ", ": ", " - "}

func splitTitleDesc(text string) (title, desc string) {
	for _, sep := range descSeparators {
		if i := strings.Index(text, sep); i >= 0 {
			return text[:i], text[i+len(sep):]
		}
	}
	return text, ""
}

// applyMeta parses a metadata body like "priority: 2, depends: f01, f02" and
// applies it to feat.
func applyMeta(feat *Feature, body string) {
	for _, part := range strings.Split(body, ",") {
		part = strings.TrimSpace(part)
		key, val, ok := strings.Cut(part, ":")
		if !ok {
			// A bare token inside the metadata block is treated as a dependency.
			if dep := strings.TrimSpace(part); dep != "" {
				feat.DependsOn = append(feat.DependsOn, dep)
			}
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "priority", "prio":
			if n, err := strconv.Atoi(val); err == nil {
				feat.Priority = n
			}
		case "depends", "dependson", "deps":
			for _, d := range strings.Fields(strings.ReplaceAll(val, ",", " ")) {
				feat.DependsOn = append(feat.DependsOn, d)
			}
		}
	}
}
