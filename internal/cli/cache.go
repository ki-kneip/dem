package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/ui"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manages DEM's download cache",
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Deletes cached downloads (verified artifacts, the registry snapshot)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		freed, err := dirSize(paths.Cache)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.RemoveAll(paths.Cache); err != nil {
			return err
		}
		if err := os.MkdirAll(paths.Cache, 0o755); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success("cache cleared"+freedSuffix(freed)))
		return nil
	},
}

// dirSize sums the size of every regular file under dir. A missing
// dir reports zero, not an error — an empty cache is a valid state.
func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}

func freedSuffix(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	return " (" + formatSize(bytes) + " freed)"
}

// formatSize renders bytes as a human-readable size (KiB, MiB...).
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	cacheCmd.AddCommand(cacheCleanCmd)
}
