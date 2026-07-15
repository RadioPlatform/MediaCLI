package cli

import (
	"github.com/spf13/cobra"

	"radioplatform-media-ci/internal/buildinfo"
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
