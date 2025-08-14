package cmd

import (
	"github.com/spf13/cobra"
)

// ciliumCmd is the parent command for Cilium-related operations
var ciliumCmd = &cobra.Command{
	Use:   "cilium",
	Short: "Cilium configuration and feature validation",
	Long:  "Commands to inspect Cilium configuration and validate feature prerequisites.",
}

func init() {
	rootCmd.AddCommand(ciliumCmd)
}
