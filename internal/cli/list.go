package cli

import (
	"fmt"
	"io"
	"strings"

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
			fmt.Fprintln(out, ui.Title(prov.Name()+" — available versions"))
			printRemoteVersions(out, paths, prov.Name(), versions, flagListLimit)
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
	listCmd.Flags().IntVar(&flagListLimit, "limit", 15, "maximum number of remote versions shown (or groups, for providers that group by vendor)")
}

// printRemoteVersions prints a "dem list <tool> --remote" result. When
// entries carry a Group (e.g. java's JDK vendors), it prints one
// section per group with the group's entries nested underneath and
// the group prefix trimmed off; limit then caps the number of groups
// shown, not the number of individual version lines, so every shown
// group keeps its full set of entries. Ungrouped providers keep the
// original flat listing, capped at limit lines.
func printRemoteVersions(out io.Writer, paths core.Paths, tool string, versions []core.Version, limit int) {
	grouped := false
	for _, v := range versions {
		if v.Group != "" {
			grouped = true
			break
		}
	}
	if !grouped {
		if len(versions) > limit {
			versions = versions[:limit]
		}
		for _, v := range versions {
			fmt.Fprintln(out, remoteVersionLine("  ", v.Raw, v, paths, tool))
		}
		return
	}

	var order []string
	byGroup := make(map[string][]core.Version)
	for _, v := range versions {
		if _, ok := byGroup[v.Group]; !ok {
			order = append(order, v.Group)
		}
		byGroup[v.Group] = append(byGroup[v.Group], v)
	}
	if len(order) > limit {
		order = order[:limit]
	}
	for _, group := range order {
		fmt.Fprintln(out, "  "+group)
		for _, v := range byGroup[group] {
			label := strings.TrimPrefix(v.Raw, group+"-")
			fmt.Fprintln(out, remoteVersionLine("    ", label, v, paths, tool))
		}
	}
}

func remoteVersionLine(indent, label string, v core.Version, paths core.Paths, tool string) string {
	line := indent + label
	if v.LTS {
		line += "  " + ui.Badge("LTS")
	}
	if paths.IsInstalled(tool, v.Raw) {
		line += "  " + ui.Dim("(installed)")
	}
	return line
}
