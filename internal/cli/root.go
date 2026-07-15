package cli

import (
	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/ui"
)

// Version is injected at build time via -ldflags "-X ...cli.Version=v0.1.0".
var Version = "dev"

var flagPlain bool

var rootCmd = &cobra.Command{
	Use:           "dem",
	Short:         "dem — Development Environment Manager (runtimes, SDKs and CLIs)",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if flagPlain {
			ui.SetPlain()
			httpx.ShowProgress = false
		}
	},
}

func Execute() error {
	rootCmd.PersistentFlags().BoolVar(&flagPlain, "plain", false,
		"plain text output, no colors or interactivity (for scripts/CI)")
	rootCmd.AddCommand(setupCmd, selfUpdateCmd, installCmd, uninstallCmd, useCmd, listCmd, currentCmd, doctorCmd, shimsCmd, relocateCmd, cacheCmd)
	return rootCmd.Execute()
}
