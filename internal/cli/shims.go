package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/shimbin"
	"github.com/ki-kneip/dem/internal/ui"
)

var shimsCmd = &cobra.Command{
	Use:   "shims",
	Short: "Manages the shims (the node, npm, go... interceptors on PATH)",
}

var shimsRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Re-extracts dem-shim and relinks every shim (used after updates)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		if err := paths.RefreshShims(shimbin.Payload); err != nil {
			return err
		}
		if err := refreshInstalledBinary(paths); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), ui.Success("shims refreshed"))
		return nil
	},
}

// refreshInstalledBinary realigns bin/dem with the current executable
// (relevant after a self-update run from elsewhere).
func refreshInstalledBinary(paths core.Paths) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := os.Stat(paths.Bin); err != nil {
		return nil // installation without bin/ (e.g. running from source)
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	if err := core.LinkOrCopy(self, filepath.Join(paths.Bin, "dem"+ext)); err != nil {
		return fmt.Errorf("updating dem: %w", err)
	}
	return nil
}

func init() {
	shimsCmd.AddCommand(shimsRefreshCmd)
}
