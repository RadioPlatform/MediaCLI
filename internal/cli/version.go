package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/buildinfo"
	"radioplatform-media-ci/internal/output"
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  `Display the version, build commit, build date, Go version, and platform.`,
		Args:  cobra.NoArgs,
		RunE:  runVersion,
	}
	return cmd
}

func runVersion(cmd *cobra.Command, args []string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"version":  buildinfo.Version,
			"commit":   buildinfo.Commit,
			"built":    buildinfo.BuildDate,
			"go":       buildinfo.GoVersion,
			"platform": platform,
		})
		return nil
	}

	out.PrintInfo(fmt.Sprintf("rpmedia-cli %s", buildinfo.Version))
	out.PrintKV("Commit", buildinfo.Commit)
	out.PrintKV("Built", buildinfo.BuildDate)
	out.PrintKV("Go", buildinfo.GoVersion)
	out.PrintKV("Platform", platform)

	return nil
}
