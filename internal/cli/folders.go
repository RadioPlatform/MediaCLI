package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

func NewFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage media folders",
		Long:  `List and create media folders on radio stations.`,
	}

	cmd.AddCommand(NewFoldersListCmd())
	cmd.AddCommand(NewFoldersCreateCmd())

	return cmd
}

func NewFoldersListCmd() *cobra.Command {
	var stationFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List media folders",
		Long: `List media folders for a station.

Examples:
  rpmedia-cli folders list
  rpmedia-cli folders list --station "Accra Radio"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFoldersList(cmd, stationFlag)
		},
	}

	cmd.Flags().StringVar(&stationFlag, "station", "", "Station UUID or name")

	return cmd
}

func NewFoldersCreateCmd() *cobra.Command {
	var stationFlag string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a media folder",
		Long: `Create a new media folder on a station.

Examples:
  rpmedia-cli folders create "New Releases"
  rpmedia-cli folders create "Jingles" --station 2f71a6cb`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFoldersCreate(cmd, args[0], stationFlag)
		},
	}

	cmd.Flags().StringVar(&stationFlag, "station", "", "Station UUID or name")

	return cmd
}

func runFoldersList(cmd *cobra.Command, stationFlag string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

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

	folders, err := client.ListFolders(cmd.Context(), station.UUID)
	if err != nil {
		return handleAPIError(out, err)
	}

	if out.IsJSON() {
		result := map[string]interface{}{
			"station_uuid": station.UUID,
			"station_name": station.Name,
			"folders":      folders,
		}
		out.PrintJSON(result)
		return nil
	}

	out.PrintKV("Station", station.Name)
	out.PrintKV("UUID", station.UUID)
	out.Println()

	if len(folders) == 0 {
		out.PrintInfo("No folders found.")
		return nil
	}

	headers := []string{"Name", "ID", "Media"}
	rows := make([][]string, len(folders))
	for i, f := range folders {
		id := ""
		if f.ID > 0 {
			id = fmt.Sprintf("%d", f.ID)
		}
		mediaCount := ""
		if f.MediaCount > 0 {
			mediaCount = fmt.Sprintf("%d", f.MediaCount)
		}
		rows[i] = []string{f.Name, id, mediaCount}
	}
	out.Table(headers, rows)

	return nil
}

func runFoldersCreate(cmd *cobra.Command, name string, stationFlag string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	if name == "" {
		return fmt.Errorf("folder name cannot be empty")
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

	existing, err := client.ListFolders(cmd.Context(), station.UUID)
	if err != nil {
		return handleAPIError(out, err)
	}

	for _, f := range existing {
		if strings.EqualFold(f.Name, name) {
			msg := fmt.Sprintf("Folder %q already exists on station %q.", name, station.Name)
			if out.IsJSON() {
				out.PrintJSONError(map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "duplicate_folder",
						"message": msg,
					},
				})
			} else {
				out.PrintError(msg)
			}
			return fmt.Errorf("folder %q already exists", name)
		}
	}

	folder, err := client.CreateFolder(cmd.Context(), station.UUID, name)
	if err != nil {
		return handleAPIError(out, err)
	}

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"success":      true,
			"station_uuid": station.UUID,
			"station_name": station.Name,
			"folder":       folder,
		})
	} else {
		out.PrintOK(fmt.Sprintf("Folder %q created on station %q.", folder.Name, station.Name))
	}

	return nil
}
