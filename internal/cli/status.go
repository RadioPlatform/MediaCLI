package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display current authentication and configuration status",
		Long:  `Display the current CLI configuration, including credentials status, default station, and server connectivity.`,
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
	return cmd
}

func NewWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display current authentication status (alias for status)",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	configPath, _ := config.ConfigPath()
	maskedKey := cfg.MaskedKey()
	credSource := cfg.CredentialSource()

	authValid := false
	stationExists := false
	connectionError := ""

	if cfg.HasAPIKey() {
		client := api.NewClient(cfg.EffectiveAPIKey())
		stations, err := client.ListStations(cmd.Context())
		if err == nil {
			authValid = true
			if cfg.HasDefaultStation() {
				for _, s := range stations {
					if s.UUID == cfg.DefaultStationUUID {
						stationExists = true
						break
					}
				}
			}
		} else if apiErr := api.AsAPIError(err); apiErr != nil {
			connectionError = apiErr.FriendlyMessage()
		} else {
			connectionError = err.Error()
		}
	}

	if out.IsJSON() {
		result := map[string]interface{}{
			"product":                "Radioplatform Media CLI",
			"server":                 api.APIBaseURL,
			"credentials":            maskedKey,
			"credential_source":      credSource,
			"config_file":            configPath,
			"default_station":        cfg.DefaultStationName,
			"default_station_uuid":   cfg.DefaultStationUUID,
			"auth_valid":             authValid,
			"default_station_exists": stationExists,
			"connection_error":       connectionError,
		}
		out.PrintJSON(result)
		return nil
	}

	out.PrintTitle("Radioplatform Media CLI")
	out.Println()
	out.PrintKV("Server", api.APIBaseURL)

	if maskedKey != "" {
		out.PrintKV("Credentials", maskedKey)
	} else {
		out.PrintKV("Credentials", "not configured")
	}
	out.PrintKV("Credential source", credSource)
	if connectionError != "" {
		out.PrintError("Server connection: " + connectionError)
	}

	if configPath != "" {
		out.PrintKV("Config file", configPath)
	}

	if cfg.HasDefaultStation() {
		out.PrintKV("Default station", cfg.DefaultStationName)
		out.PrintKV("Station UUID", cfg.DefaultStationUUID)
	} else {
		out.PrintKV("Default station", "not set")
	}

	if cfg.HasAPIKey() {
		if authValid {
			out.PrintKV("Authentication", "Valid")
		} else {
			out.PrintKV("Authentication", "Invalid")
		}
		if cfg.HasDefaultStation() {
			if stationExists {
				out.PrintKV("Default station", "Available")
			} else {
				out.PrintKV("Default station", "Not found on server")
			}
		}
	} else {
		out.PrintKV("Authentication", "Not configured")
	}

	return nil
}
