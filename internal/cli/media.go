package cli

import (
	"github.com/spf13/cobra"
)

func NewMediaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "Manage media files",
		Long:  `Upload and list media files on radio stations.`,
	}

	cmd.AddCommand(NewMediaUploadCmd())
	cmd.AddCommand(NewMediaListCmd())

	return cmd
}
