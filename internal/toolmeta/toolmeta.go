// Package toolmeta holds static metadata about the supported tools.
// It is shared between dem and dem-shim, so it must stay lean: no
// networking, TUI or external dependencies.
package toolmeta

// Executables maps each built-in tool to the binaries that get a
// shim. Registry tools (registry.yaml) declare theirs in the
// registry itself.
var Executables = map[string][]string{
	"go":   {"go", "gofmt"},
	"node": {"node", "npm", "npx", "corepack"},
}

// ToolFor finds the tool that owns an executable ("npm" -> "node").
func ToolFor(execName string) (string, bool) {
	for tool, execs := range Executables {
		for _, e := range execs {
			if e == execName {
				return tool, true
			}
		}
	}
	return "", false
}

// EnvVars returns environment variables the tool requires pointing
// at the installation directory (e.g. JAVA_HOME once the Java
// provider lands). nil when there are none.
func EnvVars(tool, installDir string) map[string]string {
	return nil
}
