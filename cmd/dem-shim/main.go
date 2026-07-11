// dem-shim is the minimal binary that lives in <root>/shims under
// the tools' names (node, npm, go...). It only resolves the active
// version and hands execution over — everything else (downloads,
// TUI, commands) stays in the main dem binary, which embeds and
// extracts this one.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ki-kneip/dem/internal/shim"
)

func main() {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe"))
	if name == "dem-shim" {
		fmt.Fprintln(os.Stderr, "dem-shim is DEM's internal shim executor and is not meant to be run directly")
		os.Exit(2)
	}
	os.Exit(shim.Run(name, os.Args[1:]))
}
