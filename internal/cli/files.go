package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func parseExts(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		out = append(out, p)
	}
	return out
}

func listDirFiles(dir, extCSV string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	want := map[string]struct{}{}
	for _, e := range parseExts(extCSV) {
		want[e] = struct{}{}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("no file extensions given")
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// WalkDir does not follow directory symlinks, so it cannot loop.
		if !d.Type().IsRegular() {
			return nil
		}
		if _, ok := want[strings.ToLower(filepath.Ext(path))]; !ok {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
