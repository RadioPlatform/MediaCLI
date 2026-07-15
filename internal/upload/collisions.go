package upload

import (
	"fmt"
	"sort"
	"strings"
)

type Collision struct {
	DestinationFolder string
	DestinationName   string
	ConflictingPaths  []string
}

func DetectCollisions(items []UploadItem) []Collision {
	keyMap := make(map[string][]string)

	for _, item := range items {
		folder := item.DestinationFolder
		name := strings.ToLower(item.DestinationName)
		key := folder + ":" + name
		keyMap[key] = append(keyMap[key], item.LocalPath)
	}

	var collisions []Collision
	for key, paths := range keyMap {
		if len(paths) > 1 {
			parts := strings.SplitN(key, ":", 2)
			collisions = append(collisions, Collision{
				DestinationFolder: parts[0],
				DestinationName:   itemNameFromPaths(paths),
				ConflictingPaths:  paths,
			})
		}
	}

	sort.Slice(collisions, func(i, j int) bool {
		if collisions[i].DestinationFolder != collisions[j].DestinationFolder {
			return collisions[i].DestinationFolder < collisions[j].DestinationFolder
		}
		return collisions[i].DestinationName < collisions[j].DestinationName
	})

	return collisions
}

func itemNameFromPaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	base := paths[0]
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' {
			return base[i+1:]
		}
	}
	return base
}

func CollisionError(collisions []Collision) error {
	var sb strings.Builder
	sb.WriteString("Destination filename collision detected:\n")
	for _, c := range collisions {
		folder := c.DestinationFolder
		if folder == "" {
			folder = "media root"
		}
		sb.WriteString(fmt.Sprintf("\n  %s/%s\n", folder, c.DestinationName))
		for _, p := range c.ConflictingPaths {
			sb.WriteString(fmt.Sprintf("    - %s\n", p))
		}
	}
	sb.WriteString("\nUse --allow-name-collisions to upload anyway (the server will determine how duplicates are handled).")
	return fmt.Errorf("%s", sb.String())
}
