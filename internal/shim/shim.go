// Package shim resolves the active version of a tool and hands
// execution over to the real binary, preserving stdio, arguments and
// exit code. It is the heart of dem-shim and must not grow heavy
// dependencies (networking, TUI) — the shim's size depends on it.
package shim

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/registry"
	"github.com/ki-kneip/dem/internal/toolmeta"
)

// Run executes the real tool and returns the process exit code.
func Run(execName string, args []string) int {
	code, err := run(execName, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dem (shim %s): %v\n", execName, err)
	}
	return code
}

func run(execName string, args []string) (int, error) {
	paths, err := core.ResolvePaths()
	if err != nil {
		return 1, err
	}
	tool, ok := toolmeta.ToolFor(execName)
	if !ok {
		// registry tools: cached registry copy, or the embedded
		// snapshot — the shim never touches the network
		tool, ok = registry.LoadLocal(paths.Cache).ToolFor(execName)
	}
	if !ok {
		return 127, fmt.Errorf("no registered tool provides %q", execName)
	}
	cfg, err := core.LoadConfig(paths.ConfigFile())
	if err != nil {
		return 1, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}

	version, source, err := core.ResolveActive(tool, cwd, cfg)
	if err != nil {
		return 1, err
	}
	dir := paths.InstallDir(tool, version)
	if !paths.IsInstalled(tool, version) {
		return 1, fmt.Errorf("%s %s is set in %s but not installed; run 'dem install %s@%s'",
			tool, version, source, tool, version)
	}
	realExe, ok := core.FindExecutable(dir, execName)
	if !ok {
		return 1, fmt.Errorf("executable %s not found in %s", execName, dir)
	}

	env := os.Environ()
	// the real binary's directory goes first on PATH so sibling
	// scripts find each other (npm needs to find the right node)
	env = append(env, "PATH="+filepath.Dir(realExe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	for k, v := range toolmeta.EnvVars(tool, dir) {
		env = append(env, k+"="+v)
	}

	cmd := exec.Command(realExe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
