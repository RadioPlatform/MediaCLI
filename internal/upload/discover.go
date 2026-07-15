package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type DiscoveredFile struct {
	AbsolutePath string
	Size         int64
	Metadata     MediaMetadata
}

// DiscoverFiles expands arguments into a sorted, deduplicated list of regular files.
// Each argument can be:
//   - A literal file path
//   - A literal directory path (walked recursively)
//   - A glob pattern (supports ** globstar)
//
// Hidden files (dotfiles) are included. Symlinks are not followed.
func DiscoverFiles(args []string) ([]DiscoveredFile, error) {
	seen := make(map[string]bool)
	var files []DiscoveredFile

	for _, arg := range args {
		arg = normalizeInputPath(arg)

		info, err := os.Stat(arg)
		if err == nil {
			if info.Mode().IsRegular() {
				abs, _ := filepath.Abs(arg)
				if !seen[abs] {
					seen[abs] = true
					files = append(files, makeDiscoveredFile(abs, info.Size()))
				}
				continue
			}
			if info.IsDir() {
				dirFiles, err := walkDirectory(arg)
				if err != nil {
					return nil, fmt.Errorf("error walking directory %q: %w", arg, err)
				}
				for _, f := range dirFiles {
					if !seen[f.AbsolutePath] {
						seen[f.AbsolutePath] = true
						files = append(files, f)
					}
				}
				continue
			}
			return nil, fmt.Errorf("%q is not a regular file or directory", arg)
		}

		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot access %q: %w", arg, err)
		}

		if !containsGlobChars(arg) {
			return nil, fmt.Errorf("path does not exist: %s", arg)
		}

		matchedFiles, err := expandGlob(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", arg, err)
		}
		if len(matchedFiles) == 0 {
			return nil, fmt.Errorf("no matches found for %q", arg)
		}

		for _, f := range matchedFiles {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.Mode().IsRegular() {
				abs, _ := filepath.Abs(f)
				if !seen[abs] {
					seen[abs] = true
					files = append(files, makeDiscoveredFile(abs, info.Size()))
				}
				continue
			}
			if info.IsDir() {
				dirFiles, err := walkDirectory(f)
				if err != nil {
					continue
				}
				for _, df := range dirFiles {
					if !seen[df.AbsolutePath] {
						seen[df.AbsolutePath] = true
						files = append(files, df)
					}
				}
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no uploadable files found")
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].AbsolutePath < files[j].AbsolutePath
	})

	return files, nil
}

func containsGlobChars(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '*', '?', '[', '{':
			return true
		}
	}
	return false
}

func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func normalizeInputPath(path string) string {
	return filepath.Clean(expandHomeDir(path))
}

func makeDiscoveredFile(path string, size int64) DiscoveredFile {
	return DiscoveredFile{
		AbsolutePath: path,
		Size:         size,
		Metadata:     readMediaMetadata(path),
	}
}

func walkDirectory(dir string) ([]DiscoveredFile, error) {
	var files []DiscoveredFile
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			abs, _ := filepath.Abs(path)
			files = append(files, makeDiscoveredFile(abs, info.Size()))
		}
		return nil
	})
	return files, err
}

func expandGlob(pattern string) ([]string, error) {
	base, pattern := doublestar.SplitPattern(pattern)
	if base == "" {
		base = "."
	}

	fs := os.DirFS(base)
	matches, err := doublestar.Glob(fs, pattern)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, filepath.Join(base, m))
	}
	return result, nil
}
