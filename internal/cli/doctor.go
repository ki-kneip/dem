package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/providers"
	"github.com/ki-kneip/dem/internal/shimbin"
	"github.com/ki-kneip/dem/internal/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Checks the health of the DEM installation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d := &diagnosis{out: cmd.OutOrStdout()}

		paths, err := core.ResolvePaths()
		if err != nil {
			return err
		}
		d.pass("root: " + paths.Root)

		checkLayout(d, paths)
		checkShimBinary(d, paths)
		checkPath(d, paths)
		checkConfiguredVersions(d, paths)
		checkShadowing(d, paths)

		fmt.Fprintln(d.out)
		if d.failures == 0 {
			fmt.Fprintln(d.out, ui.Success("everything looks good"))
			return nil
		}
		return fmt.Errorf("%d problem(s) found", d.failures)
	},
}

type diagnosis struct {
	out      io.Writer
	failures int
}

func (d *diagnosis) pass(msg string) { fmt.Fprintln(d.out, ui.Success(msg)) }
func (d *diagnosis) warn(msg string) { fmt.Fprintln(d.out, ui.Warn(msg)) }
func (d *diagnosis) fail(msg string) {
	d.failures++
	fmt.Fprintln(d.out, ui.Error(msg))
}

func checkLayout(d *diagnosis, paths core.Paths) {
	missing := false
	for _, dir := range []string{paths.Bin, paths.Shims, paths.Tools, paths.Cache} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			d.fail("missing directory " + dir + "; run 'dem setup'")
			missing = true
		}
	}
	if !missing {
		d.pass("directory layout is complete")
	}
}

func checkShimBinary(d *diagnosis, paths core.Paths) {
	target := filepath.Join(paths.Shims, core.ShimBinaryName())
	existing, err := os.ReadFile(target)
	if err != nil {
		d.warn("dem-shim not extracted yet (it appears with the first 'dem install')")
		return
	}
	if !bytes.Equal(existing, shimbin.Payload) {
		d.warn("dem-shim is stale; run 'dem shims refresh'")
		return
	}
	d.pass("dem-shim is up to date")
}

func checkPath(d *diagnosis, paths core.Paths) {
	if onPath(paths.Shims) {
		d.pass("shims directory is on PATH")
	} else {
		d.fail("shims directory is not on PATH; run 'dem setup' for instructions")
	}
	if onPath(paths.Bin) {
		d.pass("bin directory is on PATH")
	} else {
		d.warn("bin directory is not on PATH; 'dem' itself won't be callable by name")
	}
}

func checkConfiguredVersions(d *diagnosis, paths core.Paths) {
	cfg, err := core.LoadConfig(paths.ConfigFile())
	if err != nil {
		d.fail("cannot read " + paths.ConfigFile() + ": " + err.Error())
		return
	}
	for tool, version := range cfg.Global {
		if !paths.IsInstalled(tool, version) {
			d.fail(fmt.Sprintf("global %s@%s is configured but not installed; run 'dem install %s@%s'",
				tool, version, tool, version))
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	if path, pf, found, err := core.FindProjectFile(cwd); err == nil && found {
		for tool, version := range pf.Tools {
			if !paths.IsInstalled(tool, version) {
				d.fail(fmt.Sprintf("%s pins %s@%s, which is not installed; run 'dem install %s@%s'",
					path, tool, version, tool, version))
			}
		}
	}
}

// checkShadowing verifies that shimmed executables of installed tools
// actually resolve to DEM's shims, not to some other installation
// earlier on PATH (a system Node, nvm, etc.).
func checkShadowing(d *diagnosis, paths core.Paths) {
	for _, tool := range providers.Names() {
		installed, err := paths.InstalledVersions(tool)
		if err != nil || len(installed) == 0 {
			continue
		}
		prov, _ := providers.Get(tool)
		for _, name := range prov.Executables() {
			resolved, err := exec.LookPath(name)
			if err != nil {
				continue // absence is already covered by the PATH check
			}
			if !samePath(filepath.Dir(resolved), paths.Shims) {
				d.warn(fmt.Sprintf("%s resolves to %s, shadowing DEM's shim; check your PATH order", name, resolved))
			}
		}
	}
}

func onPath(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if samePath(entry, dir) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}
