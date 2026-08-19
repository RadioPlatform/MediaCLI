package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/buildinfo"
	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

var (
	jsonFlag    bool
	noColorFlag bool
	debugFlag   bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media-cli",
		Short: "Radioplatform Media CLI",
		Long: `Radioplatform Media CLI

Manage your Radio Platform station media library from the command line.

Getting started:

  media-cli login
  media-cli media upload song.mp3
  media-cli media list`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
	}

	cmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	cmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "Disable color output")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug output")

	cmd.AddCommand(NewLoginCmd())
	cmd.AddCommand(NewLogoutCmd())
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewWhoamiCmd())
	cmd.AddCommand(NewStationsCmd())
	cmd.AddCommand(NewFoldersCmd())
	cmd.AddCommand(NewMediaCmd())
	cmd.AddCommand(NewVersionCmd())

	return cmd
}

func newAPIClient(cfg *config.Config, out *output.Output) (*api.Client, error) {
	if !cfg.HasServerURL() {
		msg := "Server URL is not configured. Set RADIO_PLATFORM_CLI_URL or add server_url to the config file."
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "missing_server_url",
					"message": msg,
				},
			})
		} else {
			out.PrintError(msg)
		}
		return nil, fmt.Errorf("server URL is not configured")
	}

	return api.NewClient(cfg.EffectiveAPIKey(), api.WithBaseURL(cfg.EffectiveServerURL())), nil
}
