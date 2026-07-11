package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/ui"
)

var (
	flagListRemote bool
	flagListLimit  int
)

var listCmd = &cobra.Command{
	Use:   "list [tool]",
	Short: "Lists installed versions (or available ones, with --remote)",
	Args:  cobra.MaximumNArgs(1),
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

		if flagListRemote {
			if len(args) == 0 {
				return fmt.Errorf("--remote requires a tool: dem list node --remote")
			}
			prov, ok := providers.Get(args[0])
			if !ok {
				return fmt.Errorf("unknown tool %q", args[0])
			}
			versions, err := prov.ListRemote(cmd.Context())
			if err != nil {
				return err
			}
			if len(versions) > flagListLimit {
				versions = versions[:flagListLimit]
			}
			fmt.Fprintln(out, ui.Title(prov.Name()+" — available versions"))
			for _, v := range versions {
				line := "  " + v.Raw
				if v.LTS {
					line += "  " + ui.Badge("LTS")
				}
				if paths.IsInstalled(prov.Name(), v.Raw) {
					line += "  " + ui.Dim("(installed)")
				}
				fmt.Fprintln(out, line)
			}
			return nil
		}

		tools := providers.Names()
		if len(args) == 1 {
			tools = []string{args[0]}
		}
		empty := true
		for _, tool := range tools {
			installed, err := paths.InstalledVersions(tool)
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				continue
			}
			empty = false
			fmt.Fprintln(out, ui.Title(tool))
			for _, v := range installed {
				line := "  " + v
				if cfg.Global[tool] == v {
					line += "  " + ui.Dim("(global)")
				}
				fmt.Fprintln(out, line)
			}
		}
		if empty {
			fmt.Fprintln(out, ui.Dim("nothing installed yet; try 'dem install node@lts'"))
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&flagListRemote, "remote", false, "list the versions available at the official source")
	listCmd.Flags().IntVar(&flagListLimit, "limit", 15, "maximum number of remote versions shown")
}
