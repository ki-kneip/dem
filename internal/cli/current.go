package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/ui"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Shows the versions active in this directory and where they come from",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		cfg, err := core.LoadConfig(paths.ConfigFile())
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		var rows [][2]string
		for _, tool := range providers.Names() {
			version, source, err := core.ResolveActive(tool, cwd, cfg)
			if err != nil {
				rows = append(rows, [2]string{tool, ui.Dim("—")})
				continue
			}
			status := ""
			if !paths.IsInstalled(tool, version) {
				status = "  " + ui.Warn("not installed")
			}
			rows = append(rows, [2]string{tool, ui.Badge(version) + "  " + ui.Dim(source) + status})
		}
		fmt.Fprintln(out, ui.KeyValues(rows))
		return nil
	},
}
