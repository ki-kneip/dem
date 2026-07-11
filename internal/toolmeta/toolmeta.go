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
	"java": {
		"java", "javac", "jar", "javadoc", "javap", "jcmd", "jconsole",
		"jdb", "jdeps", "jfr", "jlink", "jmap", "jmod", "jpackage", "jps",
		"jrunscript", "jshell", "jstack", "jstat", "keytool", "jarsigner",
		"rmiregistry", "serialver",
	},
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
// at the installation directory. nil when there are none.
func EnvVars(tool, installDir string) map[string]string {
	switch tool {
	case "java":
		return map[string]string{"JAVA_HOME": installDir}
	default:
		return nil
	}
}
