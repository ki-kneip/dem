package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/ui"
	"github.com/ki-kneip/dem/internal/update"
)

var (
	flagCheckOnly  bool
	flagAllowMajor bool
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Updates DEM to the latest release (without crossing a major version)",
	Long: `Updates the DEM binary from the official releases.

By default it only updates within the same major version (no
breaking change). To accept a new major, run with --allow-major.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, ui.Dim("checking releases..."))

		rel, err := update.Latest(cmd.Context())
		if err != nil {
			return err
		}

		err = update.Guard(Version, rel.Version, flagAllowMajor)
		if errors.Is(err, update.ErrUpToDate) {
			fmt.Fprintln(out, ui.Success("already at the latest version ("+ui.Badge(Version)+")"))
			return nil
		}
		if err != nil {
			return err
		}

		if flagCheckOnly {
			fmt.Fprintln(out, ui.Warn("new version available: "+ui.Badge(rel.Version)+ui.Dim(" (current: "+Version+")")))
			fmt.Fprintln(out, ui.Dim("run 'dem self-update' to update"))
			return nil
		}

		fmt.Fprintf(out, "updating %s -> %s...\n", Version, ui.Badge(rel.Version))
		if err := update.Apply(cmd.Context(), rel); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success("updated to "+ui.Badge(rel.Version)))

		// the current process is still the old version; the binary
		// that knows how to extract the new dem-shim is the one just
		// written to disk
		exe, err := os.Executable()
		if err == nil {
			refresh := exec.Command(exe, "shims", "refresh", "--plain")
			refresh.Stdout, refresh.Stderr = out, cmd.ErrOrStderr()
			err = refresh.Run()
		}
		if err != nil {
			fmt.Fprintln(out, ui.Warn("binary updated, but the shim refresh failed; run 'dem shims refresh'"))
		}
		return nil
	},
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&flagCheckOnly, "check", false, "only check whether an update exists, without applying it")
	selfUpdateCmd.Flags().BoolVar(&flagAllowMajor, "allow-major", false, "allow updating across a major version (breaking change)")
}
