package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/ui"
)

var flagSetupDir string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Installs DEM on this machine: picks the directory, installs the binary and guides PATH setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, ui.Title("DEM — setup"))
		fmt.Fprintln(out)

		root, err := chooseRoot(cmd)
		if err != nil {
			return err
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return err
		}

		paths := core.PathsAt(root)
		if err := paths.EnsureLayout(); err != nil {
			return err
		}

		// config.yaml doubles as the marker that lets the binaries in
		// bin/ and shims/ discover the root on their own
		cfg, err := core.LoadConfig(paths.ConfigFile())
		if err != nil {
			return err
		}
		if err := cfg.Save(paths.ConfigFile()); err != nil {
			return err
		}

		if err := installBinary(paths.Bin); err != nil {
			return err
		}

		fmt.Fprintln(out, ui.Success("layout created at "+ui.Badge(paths.Root)))
		fmt.Fprintln(out, ui.Success("binary installed: "+ui.Badge("dem")))
		fmt.Fprintln(out)
		printActivation(out, paths)
		return nil
	},
}

// chooseRoot decides the installation directory: the --dir flag,
// then DEM_HOME, then a prompt (with ~/.dem as the default). Without
// an interactive terminal it assumes the default silently.
func chooseRoot(cmd *cobra.Command) (string, error) {
	if flagSetupDir != "" {
		return flagSetupDir, nil
	}
	if env := os.Getenv("DEM_HOME"); env != "" {
		return env, nil
	}
	def, err := core.DefaultRoot()
	if err != nil {
		return "", err
	}
	if !ui.IsInteractive() || flagPlain {
		return def, nil
	}
	return ui.Ask(cmd.InOrStdin(), cmd.OutOrStdout(), "Where should DEM be installed?", def)
}

// installBinary materializes the running executable as bin/dem
// (a hardlink when possible).
func installBinary(binDir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	if err := core.LinkOrCopy(self, filepath.Join(binDir, "dem"+ext)); err != nil {
		return fmt.Errorf("installing dem: %w", err)
	}
	return nil
}

func printActivation(out io.Writer, paths core.Paths) {
	fmt.Fprintln(out, "To activate, prepend to your PATH:")
	if runtime.GOOS == "windows" {
		fmt.Fprintln(out, ui.Dim("  # PowerShell (persistent, new sessions)"))
		fmt.Fprintf(out, "  [Environment]::SetEnvironmentVariable('Path', '%s;%s;' + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')\n", paths.Shims, paths.Bin)
	} else {
		fmt.Fprintln(out, ui.Dim("  # add to your ~/.bashrc or ~/.zshrc"))
		fmt.Fprintf(out, "  export PATH=\"%s:%s:$PATH\"\n", paths.Shims, paths.Bin)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.Dim("then open a new terminal and run: dem current"))
}

func init() {
	setupCmd.Flags().StringVar(&flagSetupDir, "dir", "", "installation directory (skips the interactive prompt)")
}
