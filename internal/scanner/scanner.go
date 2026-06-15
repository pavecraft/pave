// Package scanner infers feature implementation state from the codebase, to
// refine status beyond what the spec checklist records. It is a heuristic: a
// feature is treated as a candidate for "implemented" when its stable ID slug
// appears somewhere in the project's source (e.g. a referencing comment, test
// name, or doc).
package scanner

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pavecraft/pave/internal/project"
)

// skipDirs are directory names never descended into.
var skipDirs = map[string]bool{
	".git":         true,
	".pave":        true,
	"node_modules": true,
	"vendor":       true,
	".next":        true,
	"dist":         true,
	"build":        true,
}

// maxFileSize bounds how large a file the scanner will read (1 MiB).
const maxFileSize = 1 << 20

// Scan walks root and reports, per feature ID, whether that ID appears anywhere
// in the project's text files. Only feature IDs are searched, so the result is a
// conservative signal that authors have referenced the feature in code.
func Scan(root string, features []project.Feature) (map[string]bool, error) {
	ids := make([]string, 0, len(features))
	found := make(map[string]bool, len(features))
	for _, f := range features {
		if f.ID != "" {
			ids = append(ids, f.ID)
			found[f.ID] = false
		}
	}
	if len(ids) == 0 {
		return found, nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil
		}
		scanFile(path, ids, found)
		return nil
	})
	if err != nil {
		return found, err
	}
	return found, nil
}

// scanFile marks any IDs found in the file at path. It stops early once every ID
// is accounted for.
func scanFile(path string, ids []string, found map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxFileSize)
	for sc.Scan() {
		line := sc.Text()
		for _, id := range ids {
			if !found[id] && strings.Contains(line, id) {
				found[id] = true
			}
		}
	}
}

// Refine upgrades pending features to implemented when the scan found their ID
// referenced in the codebase. It never downgrades an existing status. The
// returned slice is a copy; the input is not mutated.
func Refine(features []project.Feature, found map[string]bool) []project.Feature {
	out := make([]project.Feature, len(features))
	copy(out, features)
	for i := range out {
		if out[i].Status == project.StatusPending && found[out[i].ID] {
			out[i].Status = project.StatusImplemented
		}
	}
	return out
}
