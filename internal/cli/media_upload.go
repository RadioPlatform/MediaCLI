package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/internal/upload"
	"radioplatform-media-ci/pkg/api"
)

func NewMediaUploadCmd() *cobra.Command {
	var (
		stationFlag     string
		folder          string
		jingle          bool
		concurrency     int
		createFolders   bool
		allowCollisions bool
		yes             bool
	)

	cmd := &cobra.Command{
		Use:   "upload <path...>",
		Short: "Upload media files to a station",
		Long: `Upload media files to a radio station's media library.

Accepts files, glob patterns, and directories. Directories are uploaded recursively.
Nested local directories are flattened into the mapped top-level remote folder.
Files are sent in retryable chunks. Interactive terminals display
server-confirmed byte progress for each active file.

Upload one file to the default station's media root:
  media-cli media upload song.mp3

Upload one file to another station:
  media-cli media upload song.mp3 --station "Kumasi FM"

Upload multiple files:
  media-cli media upload song1.mp3 song2.mp3

Upload files matched by a glob:
  media-cli media upload "./tracks/*.mp3"

Upload a directory recursively:
  media-cli media upload ./New-Releases

Upload multiple directories into matching remote folders:
  media-cli media upload ./Music ./Jingles

Upload a directory into a specific remote folder:
  media-cli media upload ./Music --folder "High Rotation"

Upload a directory and automatically create its remote folder:
  media-cli media upload ./Music --create-folders

Upload all discovered files as jingles:
  media-cli media upload ./Jingles --jingle

Non-interactive batch upload:
  media-cli media upload ./Music \
    --station "Accra Radio" \
    --create-folders \
    --yes \
    --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMediaUpload(cmd, args, stationFlag, folder, jingle, concurrency, createFolders, allowCollisions, yes)
		},
	}

	cmd.Flags().StringVar(&stationFlag, "station", "", "Station UUID or name")
	cmd.Flags().StringVar(&folder, "folder", "", "Destination folder name")
	cmd.Flags().BoolVar(&jingle, "jingle", false, "Mark all uploaded files as jingles")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Upload concurrency (1-20)")
	cmd.Flags().BoolVar(&createFolders, "create-folders", false, "Create missing folders automatically")
	cmd.Flags().BoolVar(&allowCollisions, "allow-name-collisions", false, "Allow destination filename collisions")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	return cmd
}

func runMediaUpload(
	cmd *cobra.Command,
	args []string,
	stationFlag string,
	folder string,
	jingle bool,
	concurrency int,
	createFolders bool,
	allowCollisions bool,
	yes bool,
) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 20 {
		concurrency = 20
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.HasAPIKey() {
		return missingCredentialsError(out)
	}

	client := api.NewClient(cfg.EffectiveAPIKey())
	resolver := NewStationResolver(client, cfg, out)

	station, err := resolver.Resolve(cmd.Context(), stationFlag)
	if err != nil {
		return err
	}

	discovered, err := upload.DiscoverFiles(args)
	if err != nil {
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "invalid_input",
					"message": err.Error(),
				},
			})
		} else {
			out.PrintError(err.Error())
		}
		return err
	}

	planOpts := upload.PlanOptions{
		StationUUID:    station.UUID,
		StationName:    station.Name,
		GlobalJingle:   jingle,
		ExplicitFolder: folder,
	}

	plan, err := upload.BuildPlan(discovered, args, planOpts)
	if err != nil {
		return err
	}

	collisions := upload.DetectCollisions(plan.Items)
	if len(collisions) > 0 && !allowCollisions {
		errMsg := upload.CollisionError(collisions).Error()
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "filename_collision",
					"message": errMsg,
					"details": collisions,
				},
			})
		} else {
			out.PrintError(errMsg)
		}
		return fmt.Errorf("filename collision detected")
	}

	if len(plan.Items) == 0 {
		msg := "No uploadable files found."
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "no_files",
					"message": msg,
				},
			})
		} else {
			out.PrintError(msg)
		}
		return fmt.Errorf("%s", msg)
	}

	requiredFolders := collectRequiredFolders(plan)

	if len(requiredFolders) > 0 {
		existing, err := client.ListFolders(cmd.Context(), station.UUID)
		if err != nil {
			return handleAPIError(out, err)
		}

		existingNames := make(map[string]bool)
		for _, f := range existing {
			existingNames[strings.ToLower(f.Name)] = true
		}

		var missingFolders []string
		for _, rf := range requiredFolders {
			if rf == "" {
				continue
			}
			if !existingNames[strings.ToLower(rf)] {
				missingFolders = append(missingFolders, rf)
			}
		}

		if len(missingFolders) > 0 {
			if createFolders {
				for _, mf := range missingFolders {
					if _, err := client.CreateFolder(cmd.Context(), station.UUID, mf); err != nil {
						return handleAPIError(out, err)
					}
					if !out.IsJSON() {
						out.PrintOK(fmt.Sprintf("Created folder %q on station %q.", mf, station.Name))
					}
				}
			} else if term.IsTerminal(int(os.Stdin.Fd())) && !out.IsJSON() && !yes {
				for _, mf := range missingFolders {
					out.PrintWarning(fmt.Sprintf("Folder %q does not exist on station %q. Create it? [Y/n] ", mf, station.Name))
					var response string
					fmt.Scanln(&response)
					response = strings.TrimSpace(strings.ToLower(response))
					if response == "" || response == "y" || response == "yes" {
						if _, err := client.CreateFolder(cmd.Context(), station.UUID, mf); err != nil {
							return handleAPIError(out, err)
						}
						out.PrintOK(fmt.Sprintf("Created folder %q on station %q.", mf, station.Name))
					} else {
						msg := fmt.Sprintf("Folder %q does not exist on station %q and was not created.", mf, station.Name)
						msg += fmt.Sprintf("\n\nRun:\n  media-cli folders create %q --station %q", mf, station.Name)
						if out.IsJSON() {
							out.PrintJSONError(map[string]interface{}{
								"success": false,
								"error": map[string]interface{}{
									"code":    "folder_not_found",
									"message": msg,
									"details": map[string]interface{}{
										"station_uuid": station.UUID,
										"station_name": station.Name,
										"folder":       mf,
									},
								},
							})
						} else {
							out.PrintError(msg)
						}
						return fmt.Errorf("folder %q does not exist and was not created", mf)
					}
				}
			} else {
				msg := fmt.Sprintf("Folder(s) required but missing: %s", strings.Join(missingFolders, ", "))
				if out.IsJSON() {
					out.PrintJSONError(map[string]interface{}{
						"success": false,
						"error": map[string]interface{}{
							"code":    "folder_not_found",
							"message": msg,
							"details": map[string]interface{}{
								"station_uuid":    station.UUID,
								"station_name":    station.Name,
								"missing_folders": missingFolders,
							},
						},
					})
				} else {
					out.PrintError(msg)
					out.PrintStdErr(fmt.Sprintf("\nRun:\n  media-cli folders create <folder> --station %q", station.Name))
					out.PrintStdErr("Or provide --create-folders to create missing folders automatically.")
				}
				return fmt.Errorf("missing required folders")
			}
		}
	}

	isSingleFile := len(plan.Items) == 1

	if !isSingleFile {
		if !out.IsJSON() && !yes && term.IsTerminal(int(os.Stdin.Fd())) {
			out.PrintTitle("Upload plan")
			out.Println()
			out.PrintKV("Station", station.Name)
			out.PrintKV("Station UUID", station.UUID)
			out.PrintKV("Files", fmt.Sprintf("%d", len(plan.Items)))
			out.PrintKV("Total size", output.FormatBytes(plan.TotalBytes))
			out.Println()
			out.PrintInfo("Destinations:")
			out.Println(upload.FormatFolderBreakdown(plan.Items))
			out.Println()
			printMetadataTable(out, plan.Items)
			out.PrintKV("Concurrency", fmt.Sprintf("%d", concurrency))
			out.Println()

			out.Print("Continue with upload? [Y/n] ")
			var response string
			fmt.Scanln(&response)
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "" && response != "y" && response != "yes" {
				out.PrintInfo("Upload cancelled.")
				return nil
			}
		}
	} else {
		if !out.IsJSON() {
			destFolder := upload.FormatDestinationFolder(plan.Items[0].DestinationFolder)
			out.PrintInfo(fmt.Sprintf("Uploading %s", plan.Items[0].DestinationName))
			out.Println()
			out.PrintKV("Station", station.Name)
			out.PrintKV("UUID", station.UUID)
			out.PrintKV("Folder", destFolder)
			printItemMetadata(out, plan.Items[0])
			out.Println()
		}
	}

	showProgress := term.IsTerminal(int(os.Stdout.Fd())) && !out.IsJSON()

	executor := upload.NewExecutor(client, concurrency, showProgress)
	summary := executor.Execute(cmd.Context(), plan)

	if len(collisions) > 0 && allowCollisions {
		if out.IsJSON() {
			out.PrintDebug("Filename collisions were allowed")
		} else {
			out.PrintWarning("Filename collisions were allowed. The server determines how duplicates are handled.")
		}
	}

	if out.IsJSON() {
		results := make([]map[string]interface{}, len(summary.Items))
		for i, r := range summary.Items {
			res := map[string]interface{}{
				"local_path":         r.Item.LocalPath,
				"relative_path":      r.Item.RelativePath,
				"destination_folder": r.Item.DestinationFolder,
				"destination_name":   r.Item.DestinationName,
				"size":               r.Item.Size,
				"jingle":             r.Item.IsJingle,
				"success":            r.Success,
			}
			if r.Item.Metadata.HasValues() {
				res["metadata"] = r.Item.Metadata
			}
			if r.Error != "" {
				res["error"] = map[string]interface{}{
					"code":    "upload_failed",
					"message": r.Error,
				}
			}
			if r.Media != nil {
				res["media"] = r.Media
			}
			results[i] = res
		}

		jsonResult := map[string]interface{}{
			"station": map[string]string{
				"uuid": summary.StationUUID,
				"name": summary.StationName,
			},
			"total_files":    len(summary.Items),
			"total_bytes":    summary.TotalBytes,
			"uploaded_bytes": summary.UploadedBytes,
			"succeeded":      summary.SuccessCount,
			"failed":         summary.FailedCount,
			"results":        results,
		}
		out.PrintJSON(jsonResult)
	} else {
		out.PrintTitle("Upload complete")
		out.Println()
		out.PrintKV("Station", summary.StationName)
		out.PrintKV("UUID", summary.StationUUID)
		out.Println()
		out.PrintInfo(fmt.Sprintf("%d files processed", len(summary.Items)))
		out.PrintOK(fmt.Sprintf("%d succeeded", summary.SuccessCount))
		if summary.FailedCount > 0 {
			out.PrintError(fmt.Sprintf("%d failed", summary.FailedCount))
		}
		out.PrintKV("Uploaded", output.FormatBytes(summary.UploadedBytes))
		out.Println()

		if summary.FailedCount > 0 {
			out.PrintTitle("Failures:")
			for _, r := range summary.Items {
				if !r.Success {
					out.PrintStdErr(fmt.Sprintf("  %s", r.Item.RelativePath))
					out.PrintStdErr(fmt.Sprintf("    %s", r.Error))
					out.Println()
				}
			}
		}
	}

	if summary.FailedCount > 0 {
		return fmt.Errorf("upload completed with %d failure(s)", summary.FailedCount)
	}

	return nil
}

func collectRequiredFolders(plan *upload.UploadPlan) []string {
	seen := make(map[string]bool)
	var folders []string
	for _, item := range plan.Items {
		f := item.DestinationFolder
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		folders = append(folders, f)
	}
	return folders
}

const metadataPreviewLimit = 25

func printMetadataTable(out *output.Output, items []upload.UploadItem) {
	hasMetadata := false
	for _, item := range items {
		if item.Metadata.HasValues() {
			hasMetadata = true
			break
		}
	}
	if !hasMetadata {
		return
	}

	limit := len(items)
	if limit > metadataPreviewLimit {
		limit = metadataPreviewLimit
	}
	rows := make([][]string, 0, limit)
	for _, item := range items[:limit] {
		rows = append(rows, []string{
			item.RelativePath,
			metadataValue(item.Metadata.Title),
			metadataValue(item.Metadata.Artist),
			metadataValue(item.Metadata.Album),
			formatTrack(item.Metadata),
		})
	}

	out.PrintInfo("Embedded media metadata:")
	out.Table([]string{"File", "Title", "Artist", "Album", "Track"}, rows)
	if len(items) > limit {
		out.PrintInfo(fmt.Sprintf("... and %d more files", len(items)-limit))
	}
	out.Println()
}

func printItemMetadata(out *output.Output, item upload.UploadItem) {
	metadata := item.Metadata
	if !metadata.HasValues() {
		out.PrintKV("Metadata", "No embedded tags found")
		return
	}
	if metadata.Title != "" {
		out.PrintKV("Title", metadata.Title)
	}
	if metadata.Artist != "" {
		out.PrintKV("Artist", metadata.Artist)
	}
	if metadata.Album != "" {
		out.PrintKV("Album", metadata.Album)
	}
	if metadata.AlbumArtist != "" && metadata.AlbumArtist != metadata.Artist {
		out.PrintKV("Album artist", metadata.AlbumArtist)
	}
	if track := formatTrack(metadata); track != "-" {
		out.PrintKV("Track", track)
	}
	if metadata.Year != 0 {
		out.PrintKV("Year", strconv.Itoa(metadata.Year))
	}
	if metadata.Genre != "" {
		out.PrintKV("Genre", metadata.Genre)
	}
	if metadata.TagFormat != "" {
		out.PrintKV("Tags", metadata.TagFormat)
	}
}

func metadataValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func formatTrack(metadata upload.MediaMetadata) string {
	if metadata.Track == 0 {
		return "-"
	}
	if metadata.TrackTotal > 0 {
		return fmt.Sprintf("%d/%d", metadata.Track, metadata.TrackTotal)
	}
	return strconv.Itoa(metadata.Track)
}
