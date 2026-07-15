package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type UploadItem struct {
	LocalPath         string
	RelativePath      string
	DestinationFolder string
	DestinationName   string
	IsJingle          bool
	Size              int64
	Metadata          MediaMetadata
}

type UploadPlan struct {
	StationUUID    string
	StationName    string
	Items          []UploadItem
	TotalBytes     int64
	GlobalJingle   bool
	ExplicitFolder string
}

type PlanOptions struct {
	StationUUID    string
	StationName    string
	GlobalJingle   bool
	ExplicitFolder string
}

// BuildPlan creates an upload plan from discovered files.
// It maps files to destination folders based on the input arguments and options.
//
// Rules:
//   - If ExplicitFolder is set, all files go to that folder.
//   - If a top-level directory was supplied, its basename becomes the destination folder.
//   - Direct file arguments map to media root (empty folder).
//   - Glob-resolved files map to media root (empty folder).
//   - Nested local directories are flattened into the mapped folder.
func BuildPlan(discoveredFiles []DiscoveredFile, args []string, opts PlanOptions) (*UploadPlan, error) {
	if len(discoveredFiles) == 0 {
		return nil, fmt.Errorf("no files to upload")
	}

	// Determine the top-level directories from the original args
	dirs := extractDirectories(args)

	plan := &UploadPlan{
		StationUUID:    opts.StationUUID,
		StationName:    opts.StationName,
		GlobalJingle:   opts.GlobalJingle,
		ExplicitFolder: opts.ExplicitFolder,
	}

	fileDirMap := make(map[string]string) // absolute path -> destination folder

	if opts.ExplicitFolder != "" {
		for _, f := range discoveredFiles {
			fileDirMap[f.AbsolutePath] = opts.ExplicitFolder
		}
	} else if len(dirs) > 0 {
		// Map files to folders based on which top-level directory they're under.
		// For files directly specified (not under a directory arg), they go to media root.
		for _, f := range discoveredFiles {
			folder := ""
			for _, d := range dirs {
				dAbs, _ := filepath.Abs(d)
				if strings.HasPrefix(f.AbsolutePath, dAbs+string(filepath.Separator)) {
					folder = filepath.Base(dAbs)
					break
				}
				if f.AbsolutePath == dAbs {
					folder = filepath.Base(dAbs)
					break
				}
			}
			fileDirMap[f.AbsolutePath] = folder
		}
	} else {
		// No directories, no explicit folder - media root for all
		for _, f := range discoveredFiles {
			fileDirMap[f.AbsolutePath] = ""
		}
	}

	relDirs := extractDirectories(args)

	for _, f := range discoveredFiles {
		relPath := filepath.Base(f.AbsolutePath)

		if len(relDirs) > 0 {
			for _, d := range relDirs {
				dAbs, _ := filepath.Abs(d)
				dAbs = filepath.Clean(dAbs)
				if strings.HasPrefix(f.AbsolutePath, dAbs+string(filepath.Separator)) {
					relPath = f.AbsolutePath[len(dAbs)+1:]
					break
				}
				if f.AbsolutePath == dAbs {
					relPath = filepath.Base(f.AbsolutePath)
					break
				}
			}
		}

		folder := fileDirMap[f.AbsolutePath]

		item := UploadItem{
			LocalPath:         f.AbsolutePath,
			RelativePath:      relPath,
			DestinationFolder: folder,
			DestinationName:   filepath.Base(f.AbsolutePath),
			IsJingle:          opts.GlobalJingle,
			Size:              f.Size,
			Metadata:          f.Metadata,
		}
		plan.Items = append(plan.Items, item)
		plan.TotalBytes += f.Size
	}

	sort.Slice(plan.Items, func(i, j int) bool {
		return plan.Items[i].LocalPath < plan.Items[j].LocalPath
	})

	return plan, nil
}

func extractDirectories(args []string) []string {
	var dirs []string
	for _, arg := range args {
		arg = normalizeInputPath(arg)
		info, err := filepath.EvalSymlinks(arg)
		if err != nil {
			continue
		}
		fi, err := os.Stat(info)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, arg)
		}
	}
	return dirs
}
