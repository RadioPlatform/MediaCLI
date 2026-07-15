package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

func NewMediaListCmd() *cobra.Command {
	var (
		stationFlag string
		folder      string
		search      string
		page        int
		perPage     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List media files on a station",
		Long: `List media files on a radio station.

Examples:
  media-cli media list
  media-cli media list --station "Accra Radio"
  media-cli media list --folder "High Rotation"
  media-cli media list --search "afrobeats"
  media-cli media list --page 2 --per-page 100
  media-cli media list --search "station id" --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMediaList(cmd, stationFlag, folder, search, page, perPage)
		},
	}

	cmd.Flags().StringVar(&stationFlag, "station", "", "Station UUID or name")
	cmd.Flags().StringVar(&folder, "folder", "", "Filter by folder name")
	cmd.Flags().StringVar(&search, "search", "", "Search by filename or title (case-insensitive, client-side)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based)")
	cmd.Flags().IntVar(&perPage, "per-page", 50, "Results per page")

	return cmd
}

func runMediaList(
	cmd *cobra.Command,
	stationFlag string,
	folder string,
	search string,
	page int,
	perPage int,
) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
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

	params := api.MediaListParams{
		Page:    page,
		PerPage: perPage,
		Folder:  folder,
	}

	resp, err := client.ListMedia(cmd.Context(), station.UUID, params)
	if err != nil {
		return handleAPIError(out, err)
	}

	items := resp.Data

	if search != "" {
		var filtered []api.MediaItem
		lowerSearch := strings.ToLower(search)
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Filename), lowerSearch) ||
				strings.Contains(strings.ToLower(item.Title), lowerSearch) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	if out.IsJSON() {
		result := map[string]interface{}{
			"station_uuid": station.UUID,
			"station_name": station.Name,
			"media":        items,
			"page":         resp.Meta.CurrentPage,
			"per_page":     resp.Meta.PerPage,
			"total":        resp.Meta.Total,
			"last_page":    resp.Meta.LastPage,
		}
		out.PrintJSON(result)
		return nil
	}

	out.PrintKV("Station", station.Name)
	out.PrintKV("UUID", station.UUID)
	out.Println()

	if len(items) == 0 {
		out.PrintInfo("No media files found.")
		return nil
	}

	headers := []string{"Filename", "Title", "Folder", "Size", "Duration", "Jingle"}
	rows := make([][]string, len(items))
	for i, m := range items {
		folderName := m.Folder
		if folderName == "" {
			folderName = "-"
		}
		title := m.Title
		if title == "" {
			title = "-"
		}
		duration := "-"
		if m.Duration > 0 {
			duration = output.FormatDuration(m.Duration)
		}
		jingleStr := output.FormatBool(m.IsJingle)

		rows[i] = []string{
			m.Filename,
			title,
			folderName,
			output.FormatBytes(m.Size),
			duration,
			jingleStr,
		}
	}
	out.Table(headers, rows)

	if resp.Meta.Total > 0 {
		out.Println()
		out.PrintKV("Page", fmt.Sprintf("%d of %d", resp.Meta.CurrentPage, resp.Meta.LastPage))
		out.PrintKV("Total", fmt.Sprintf("%d records", resp.Meta.Total))
	}

	return nil
}
