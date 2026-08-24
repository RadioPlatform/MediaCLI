package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/buildinfo"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/internal/updater"
)

func NewUpdateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest release",
		Long: `Check GitHub for the latest stable Media CLI release and install it.

Use --check to report whether an update is available without changing the
installed executable.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check whether an update is available")

	return cmd
}

func runUpdate(cmd *cobra.Command, checkOnly bool) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)
	client := updater.NewClient()
	release, err := client.Latest(cmd.Context())
	if err != nil {
		return handleUpdateError(out, err)
	}

	comparison := updater.CompareVersions(buildinfo.Version, release.TagName)
	updateAvailable := comparison < 0
	if checkOnly {
		if out.IsJSON() {
			out.PrintJSON(map[string]interface{}{
				"current_version":  buildinfo.Version,
				"latest_version":   release.TagName,
				"update_available": updateAvailable,
				"release_url":      release.HTMLURL,
			})
			return nil
		}
		if updateAvailable {
			out.PrintInfo(fmt.Sprintf("Update available: %s → %s", buildinfo.Version, release.TagName))
			out.PrintInfo("Run media-cli update to install it.")
			return nil
		}
		out.PrintOK(fmt.Sprintf("media-cli is up to date (%s).", buildinfo.Version))
		return nil
	}

	if !updateAvailable {
		if out.IsJSON() {
			out.PrintJSON(map[string]interface{}{
				"success":         true,
				"updated":         false,
				"current_version": buildinfo.Version,
				"latest_version":  release.TagName,
			})
			return nil
		}
		out.PrintOK(fmt.Sprintf("media-cli is already up to date (%s).", buildinfo.Version))
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return handleUpdateError(out, fmt.Errorf("locate current executable: %w", err))
	}
	if !out.IsJSON() {
		out.PrintInfo(fmt.Sprintf("Updating media-cli %s → %s…", buildinfo.Version, release.TagName))
	}
	if err := client.Install(cmd.Context(), *release, executable); err != nil {
		return handleUpdateError(out, err)
	}

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"success":          true,
			"updated":          true,
			"previous_version": buildinfo.Version,
			"current_version":  release.TagName,
		})
		return nil
	}
	out.PrintOK(fmt.Sprintf("Updated media-cli to %s. Run media-cli version to confirm.", release.TagName))
	return nil
}

func handleUpdateError(out *output.Output, err error) error {
	if out.IsJSON() {
		out.PrintJSONError(map[string]interface{}{
			"success": false,
			"error": map[string]string{
				"code":    "update_failed",
				"message": err.Error(),
			},
		})
	} else {
		out.PrintError("Update failed: " + err.Error())
	}
	return err
}
