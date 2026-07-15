package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/shimbin"
	"github.com/ki-kneip/dem/internal/ui"
)

var flagRelocateYes bool

var relocateCmd = &cobra.Command{
	Use:   "relocate <new-dir>",
	Short: "Moves the DEM installation to a new directory and updates PATH",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		oldPaths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		newRoot, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		if newRoot == oldPaths.Root {
			return fmt.Errorf("dem is already installed at %s", newRoot)
		}
		empty, err := dirEmptyOrMissing(newRoot)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("%s already exists and is not empty", newRoot)
		}

		fmt.Fprintln(out, ui.Title("DEM — relocate"))
		fmt.Fprintln(out)
		fmt.Fprintf(out, "This moves %s to %s\n", ui.Badge(oldPaths.Root), ui.Badge(newRoot))
		fmt.Fprintln(out, ui.Dim("and deletes everything left at the old location."))
		fmt.Fprintln(out)

		if !flagRelocateYes {
			if !ui.IsInteractive() || flagPlain {
				return fmt.Errorf("refusing to relocate without confirmation in a non-interactive session; pass --yes to proceed")
			}
			ok, err := ui.Confirm(cmd.InOrStdin(), out, "Continue?")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, ui.Dim("cancelled"))
				return nil
			}
			fmt.Fprintln(out)
		}

		newPaths := core.PathsAt(newRoot)
		if err := newPaths.EnsureLayout(); err != nil {
			return err
		}

		shimNames, err := oldPaths.ShimNames()
		if err != nil {
			return err
		}
		if err := oldPaths.MoveTo(newPaths); err != nil {
			return fmt.Errorf("moving installation: %w", err)
		}
		if len(shimNames) > 0 {
			if err := newPaths.EnsureShims(shimbin.Payload, shimNames); err != nil {
				return err
			}
		}
		if err := installBinary(newPaths.Bin); err != nil {
			return err
		}
		fmt.Fprintln(out, ui.Success("moved to "+ui.Badge(newPaths.Root)))

		switch configured, perr := persistPath(newPaths); {
		case perr != nil:
			fmt.Fprintln(out, ui.Dim("could not update PATH automatically ("+perr.Error()+"); add it manually:"))
			printActivation(out, newPaths)
		case configured:
			fmt.Fprintln(out, ui.Success("PATH configured"))
		default:
			printActivation(out, newPaths)
		}
		if err := unpersistPath(oldPaths); err != nil {
			fmt.Fprintln(out, ui.Dim("could not remove the old PATH entry automatically: "+err.Error()))
		}

		removeOldRoot(out, oldPaths)

		fmt.Fprintln(out, ui.Dim("open a new terminal and run: dem current"))
		return nil
	},
}

// dirEmptyOrMissing reports whether dir does not exist or exists but
// has no entries — the only cases relocate accepts as a destination,
// so it never merges into or overwrites something unrelated.
func dirEmptyOrMissing(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// removeOldRoot deletes everything left at the previous installation.
// The dem binary that is currently executing this command may still
// live there — Windows in particular locks a running .exe against
// deletion — so a partial failure is reported rather than treated as
// fatal; whatever could not be removed is safe to delete by hand once
// the process using it exits.
func removeOldRoot(out io.Writer, old core.Paths) {
	if err := os.RemoveAll(old.Root); err != nil {
		fmt.Fprintln(out, ui.Dim("could not fully remove "+old.Root+": "+err.Error()+" (safe to delete by hand once this terminal is closed)"))
		return
	}
	fmt.Fprintln(out, ui.Success("removed the old installation at "+ui.Badge(old.Root)))
}

func init() {
	relocateCmd.Flags().BoolVarP(&flagRelocateYes, "yes", "y", false, "skip the confirmation prompt")
}
