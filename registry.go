// Package dem exposes repository-level assets that get embedded into
// the binaries.
package dem

import _ "embed"

// RegistryYAML is the tool registry snapshot bundled at build time.
// It is the fallback when no fetched copy is cached, and the baseline
// a fresh installation starts from.
//
//go:embed registry.yaml
var RegistryYAML []byte
