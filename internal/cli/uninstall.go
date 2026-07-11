package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/ui"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <tool>@<version>",
	Short: "Removes an installed tool version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		if !strings.Contains(args[0], "@") {
			return fmt.Errorf("specify the version to uninstall, e.g. 'dem uninstall node@22'")
		}
		tool, spec := splitToolSpec(args[0])
		prov, ok := providers.Get(tool)
		if !ok {
			return fmt.Errorf("unknown tool %q", tool)
		}
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}

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
			return fmt.Errorf("no installed version of %s matches %q", tool, spec)
		}
		id := tool + "@" + v.Raw

		if err := os.RemoveAll(paths.InstallDir(tool, v.Raw)); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success(ui.Badge(id)+" removed"))

		// drop the global default if it pointed at the removed version
		cfg, err := core.LoadConfig(paths.ConfigFile())
		if err != nil {
			return err
		}
		if cfg.Global[tool] == v.Raw {
			delete(cfg.Global, tool)
			if err := cfg.Save(paths.ConfigFile()); err != nil {
				return err
			}
			fmt.Fprintln(out, ui.Warn(id+" was the global default; set another with 'dem use "+tool+"@<version> --global'"))
		}

		// last version gone: clean up the tool directory and its shims
		remaining, err := paths.InstalledVersions(tool)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			os.RemoveAll(filepath.Join(paths.Tools, tool))
			removeShims(paths, prov.Executables())
			fmt.Fprintln(out, ui.Dim("no versions of "+tool+" left; its shims were removed"))
		}
		return nil
	},
}

func removeShims(paths core.Paths, names []string) {
	for _, name := range names {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		os.Remove(filepath.Join(paths.Shims, name))
	}
}
