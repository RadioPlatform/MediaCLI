package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

func NewStationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stations",
		Short: "List and manage radio stations",
		Long:  `List accessible stations and set the default station for CLI commands.`,
	}

	cmd.AddCommand(NewStationsListCmd())
	cmd.AddCommand(NewStationsUseCmd())

	return cmd
}

func NewStationsListCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accessible stations",
		Long:  `List all stations accessible with the current CLI API key.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStationsList(cmd, verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Display complete station UUIDs")

	return cmd
}

func NewStationsUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <uuid-or-name>",
		Short: "Set the default station",
		Long: `Set the default station for CLI commands.

The argument can be a full station UUID, a unique UUID prefix,
an exact station name, or a unique partial name match.

Examples:
  media-cli stations use 2f71a6cb
  media-cli stations use "Accra Radio"
  media-cli stations use accra`,
		Args: cobra.ExactArgs(1),
		RunE: runStationsUse,
	}

	return cmd
}

func runStationsList(cmd *cobra.Command, verbose bool) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.HasAPIKey() {
		return missingCredentialsError(out)
	}

	client := api.NewClient(cfg.EffectiveAPIKey())
	stations, err := client.ListStations(cmd.Context())
	if err != nil {
		return handleAPIError(out, err)
	}

	if out.IsJSON() {
		result := make([]map[string]string, len(stations))
		for i, s := range stations {
			result[i] = map[string]string{
				"uuid": s.UUID,
				"name": s.Name,
			}
		}
		out.PrintJSON(result)
		return nil
	}

	if len(stations) == 0 {
		out.PrintInfo("No stations found.")
		return nil
	}

	headers := []string{"Name", "UUID"}
	rows := make([][]string, len(stations))
	for i, s := range stations {
		uuid := s.UUID
		if !verbose {
			uuid = output.TruncateUUID(uuid)
		}
		rows[i] = []string{s.Name, uuid}
	}
	out.Table(headers, rows)

	return nil
}

func runStationsUse(cmd *cobra.Command, args []string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)
	input := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.HasAPIKey() {
		return missingCredentialsError(out)
	}

	client := api.NewClient(cfg.EffectiveAPIKey())
	stations, err := client.ListStations(cmd.Context())
	if err != nil {
		return handleAPIError(out, err)
	}

	station, err := resolveStationByIDOrName(stations, input)
	if err != nil {
		errMsg := err.Error()
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "ambiguous_station",
					"message": errMsg,
				},
			})
		} else {
			if strings.Contains(errMsg, "Multiple stations match") {
				out.PrintError(errMsg)
			} else {
				out.PrintError(errMsg)
			}
		}
		return err
	}

	cfg.DefaultStationUUID = station.UUID
	cfg.DefaultStationName = station.Name

	if err := config.SaveWithPreservation(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"success":      true,
			"station_uuid": station.UUID,
			"station_name": station.Name,
		})
	} else {
		out.PrintOK(fmt.Sprintf("Default station set to %s", station.Name))
	}

	return nil
}

func missingCredentialsError(out *output.Output) error {
	msg := `No CLI API key is configured.

Run:
  media-cli login

Generate a key in Account Settings -> CLI API keys.`
	if out.IsJSON() {
		out.PrintJSONError(map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    "missing_credentials",
				"message": "No CLI API key is configured.",
				"details": map[string]interface{}{
					"suggested_commands": []string{
						"media-cli login",
					},
				},
			},
		})
	} else {
		out.PrintStdErr(msg)
	}
	return fmt.Errorf("no CLI API key is configured")
}

func handleAPIError(out *output.Output, err error) error {
	if ae := api.AsAPIError(err); ae != nil {
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    string(ae.Code),
					"message": ae.FriendlyMessage(),
				},
			})
		} else {
			out.PrintError(ae.FriendlyMessage())
		}
		return err
	}
	if out.IsJSON() {
		out.PrintJSONError(map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    "unknown",
				"message": err.Error(),
			},
		})
	} else {
		out.PrintError(err.Error())
	}
	return err
}
