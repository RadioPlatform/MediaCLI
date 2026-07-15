package cli

import (
	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
)

func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials and default station",
		Long:  `Remove the stored CLI API key and default station from the configuration file.`,
		Args:  cobra.NoArgs,
		RunE:  runLogout,
	}
	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	if err := config.ClearCredentials(); err != nil {
		return err
	}

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"success":    true,
			"logged_out": true,
		})
	} else {
		out.PrintOK("\u2713 Logged out")
		out.PrintOK("\u2713 Removed stored credentials and default station")
	}

	return nil
}
