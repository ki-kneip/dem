// Package shimbin embeds the dem-shim binary for the current
// platform inside the main dem binary.
//
// Two-stage build: the go:generate directive below compiles dem-shim
// into payload/, and only then can dem itself compile (go:embed
// requires the file to exist). Always build with:
//
//	go generate ./... && go build ./...
//
// When cross-compiling, run generate with the same GOOS/GOARCH as
// the final build so the embedded payload matches the target
// platform.
package shimbin

import _ "embed"

//go:generate go build -trimpath -ldflags "-s -w" -o payload/dem-shim.bin github.com/ki-kneip/dem/cmd/dem-shim

//go:embed payload/dem-shim.bin
var Payload []byte
