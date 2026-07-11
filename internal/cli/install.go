package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/shimbin"
	"github.com/ki-kneip/dem/internal/ui"
)

var installCmd = &cobra.Command{
	Use:   "install <tool>[@version]",
	Short: "Installs a tool (e.g. dem install node@lts, dem install go@1.22)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		tool, spec := splitToolSpec(args[0])
		prov, ok := providers.Get(tool)
		if !ok {
			return fmt.Errorf("unknown tool %q; available: %s", tool, strings.Join(providers.Names(), ", "))
		}
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		if err := paths.EnsureLayout(); err != nil {
			return err
		}

		v, err := prov.Resolve(cmd.Context(), spec)
		if err != nil {
			return err
		}
		id := tool + "@" + v.Raw

		if paths.IsInstalled(tool, v.Raw) {
			fmt.Fprintln(out, ui.Success(ui.Badge(id)+" is already installed"))
			return paths.EnsureShims(shimbin.Payload, prov.Executables())
		}

		art, err := prov.Download(cmd.Context(), v, paths.Cache)
		if err != nil {
			return err
		}
		dir := paths.InstallDir(tool, v.Raw)
		if err := prov.Install(art, dir); err != nil {
			os.RemoveAll(dir) // do not leave a half-finished installation
			return err
		}
		if err := paths.EnsureShims(shimbin.Payload, prov.Executables()); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success(ui.Badge(id)+" installed"))

		// the first installed version of a tool becomes the global default
		cfg, err := core.LoadConfig(paths.ConfigFile())
		if err != nil {
			return err
		}
		if cfg.Global[tool] == "" {
			cfg.Global[tool] = v.Raw
			if err := cfg.Save(paths.ConfigFile()); err != nil {
				return err
			}
			fmt.Fprintln(out, ui.Dim("set as the global default (change it with 'dem use')"))
		}
		return nil
	},
}

// splitToolSpec splits "node@22" into ("node", "22"); without an @,
// the spec is latest.
func splitToolSpec(arg string) (tool, spec string) {
	parts := strings.SplitN(arg, "@", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[0], parts[1]
	}
	return parts[0], "latest"
}
