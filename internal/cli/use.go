package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/ui"
)

var flagUseGlobal bool

var useCmd = &cobra.Command{
	Use:   "use <tool>@<version>",
	Short: "Sets the active version for the current project (or globally with --global)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		tool, spec := splitToolSpec(args[0])
		if _, ok := providers.Get(tool); !ok {
			return fmt.Errorf("unknown tool %q", tool)
		}
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}

		// resolve against what is installed, not against the remote
		installed, err := paths.InstalledVersions(tool)
		if err != nil {
			return err
		}
		versions := make([]core.Version, len(installed))
		for i, raw := range installed {
			versions[i] = core.Version{Raw: raw}
		}
		v, err := core.MatchSpec(spec, versions)
		if err != nil {
			return fmt.Errorf("no installed version of %s matches %q; run 'dem install %s@%s'", tool, spec, tool, spec)
		}
		id := tool + "@" + v.Raw

		if flagUseGlobal {
			cfg, err := core.LoadConfig(paths.ConfigFile())
			if err != nil {
				return err
			}
			cfg.Global[tool] = v.Raw
			if err := cfg.Save(paths.ConfigFile()); err != nil {
				return err
			}
			fmt.Fprintln(out, ui.Success(ui.Badge(id)+" set as the global default"))
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		// update the nearest dem.yaml, or create one in the current directory
		path, pf, found, err := core.FindProjectFile(cwd)
		if err != nil {
			return err
		}
		if !found {
			path = filepath.Join(cwd, core.ProjectFileName)
		}
		if pf.Tools == nil {
			pf.Tools = map[string]string{}
		}
		pf.Tools[tool] = v.Raw
		if err := core.SaveProjectFile(path, pf); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success(ui.Badge(id)+" set in "+ui.Dim(path)))
		return nil
	},
}

func init() {
	useCmd.Flags().BoolVar(&flagUseGlobal, "global", false, "set as the global default instead of per project")
}
