# dem

**Development Environment Manager** — a single binary that installs and manages
versions of your runtimes, SDKs and CLIs (Go, Node.js, Java, pnpm — more to
come), per project or globally, on Windows, Linux and macOS.

```console
$ dem install node@lts
✓ node@24.18.0 installed

$ dem use node@24 --global
✓ node@24.18.0 set as the global default

$ node --version
v24.18.0
```

Java works the same way, with an optional vendor prefix (defaults to
Temurin):

```console
$ dem install java@21
✓ java@temurin-21.0.4+7 installed

$ dem install java@corretto-21
✓ java@corretto-21.0.4.7.1 installed

$ dem use java@corretto-21 --global
✓ java@corretto-21.0.4.7.1 set as the global default

$ java -version
openjdk version "21.0.4" ...
```

## Why

Every runtime has its own version manager (nvm, sdkman, pyenv, gvm...), each
with its own shell hacks, config files and platform quirks. `dem` is one tool,
one config format and one mental model for all of them — with first-class
Windows support, no shell integration required.

## Install

Download the binary for your platform from the
[latest release](https://github.com/ki-kneip/dem/releases/latest), then run:

```console
$ dem setup
Where should DEM be installed? [~/.dem]
```

`setup` creates the layout, installs the binary and prints the exact `PATH`
change for your shell. Open a new terminal and you are done. `dem doctor`
verifies everything at any time.

## Usage

| Command | What it does |
|---|---|
| `dem install node@lts` | Installs a version (`lts`, `latest`, `22`, `22.5.1`...) |
| `dem uninstall node@22` | Removes an installed version |
| `dem use go@1.22` | Pins the version for the current project (`dem.yaml`) |
| `dem use go@1.22 --global` | Sets the global default |
| `dem current` | Shows the active versions here, and where they come from |
| `dem list [tool] [--remote]` | Lists installed (or available) versions |
| `dem doctor` | Health check: layout, PATH, stale shims, shadowed tools |
| `dem self-update` | Updates dem itself (never crosses a major silently) |
| `dem shims refresh` | Re-extracts and relinks the shims |
| `dem relocate <dir>` | Moves the installation to `<dir>`, updates PATH, removes the old one |
| `dem cache clean` | Deletes cached downloads (frees disk, forces a re-download next install) |

Every command supports `--plain` for scripts and CI: no colors, no prompts,
no progress bars.

### Per-project versions

Drop a `dem.yaml` at your project root (or let `dem use` write it for you):

```yaml
tools:
  node: 22.5.1
  go: 1.22.5
  java: corretto-21.0.4.7.1
```

Any tool run from inside the project — any subdirectory — uses these
versions. Outside, the global defaults apply. `dem current` always tells you
which version is active and which file decided it.

## Supported tools

Built-in runtime providers:

| Tool | Source | Notes |
|---|---|---|
| `go` | go.dev/dl | `go`, `gofmt` |
| `node` | nodejs.org/dist | `node`, `npm`, `npx`, `corepack`; `lts` spec supported |
| `java` | [foojay Disco API](https://api.foojay.io) | `java`, `javac`, `jar`... multi-distribution — Temurin (default), Corretto, Zulu, GraalVM, Liberica...; select a vendor with `java@<vendor>-<version>`, e.g. `java@corretto-21` |

Registry tools — CLIs shipped as a single binary or archive on GitHub
Releases, described declaratively in [registry.yaml](registry.yaml):

| Tool | Source |
|---|---|
| `pnpm` | pnpm/pnpm (standalone, independent of the Node version) |
| `kit` | ki-kneip/kit |
| `wails` | wailsapp/wails (built locally with `go install`; requires `go` — see below) |

A registry snapshot is embedded in the binary; dem refreshes it from
this repository at most once a day, caches the copy, and only adopts
an update when its schema is supported by the running build — newer
registry formats never break older installations. Adding a tool to
the registry is a small YAML pull request, no Go required.

Some registry tools are not distributed as GitHub Release binaries and
are instead built locally with `go install` (`wails`, for example).
These declare `type: go-install` in the registry, along with the
dependencies they need (e.g. `go>=1.21`, checked before building) —
`dem` fails fast with an actionable message if they are missing. Each
build runs in its own throwaway module and build cache, removed as
soon as the compiled binary is extracted, so a tool you install once
never leaves its dependency graph behind on disk.

Planned: Python, Gradle, Maven.

## How it works

```
~/.dem/
├── bin/dem              # the CLI
├── shims/               # dem-shim + one hardlink per tool executable
├── tools/<tool>/<ver>/  # isolated installations
├── cache/               # verified downloads (sha256)
└── config.yaml          # global defaults
```

The names on your `PATH` (`node`, `npm`, `go`...) are hardlinks to
**dem-shim**, a minimal (~2.5 MB) dispatcher embedded inside the main binary.
When invoked, it resolves the active version — nearest `dem.yaml` walking up,
then the global config — and hands execution to the real binary, preserving
stdio, arguments and exit code. Installing a version never touches another;
switching versions moves no files.

Downloads are checksum-verified against each source's official manifest and
cached. `self-update` replaces the binary atomically and refuses to cross a
major version unless you pass `--allow-major`.

## Building from source

Two-stage build — the shim payload must be generated before the main binary
can embed it:

```console
$ go generate ./...   # compiles dem-shim into internal/shimbin/payload/
$ go build ./cmd/dem
```

When cross-compiling, run both steps with the same `GOOS`/`GOARCH` so the
embedded shim matches the target platform.

## License

[MIT](LICENSE)
