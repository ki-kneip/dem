package core

import "context"

// Version is a concrete, already-resolved version of a tool.
type Version struct {
	// Raw is the tool's canonical version string (e.g. "21.0.4+7"
	// for Java, "22.5.1" for Node). Comparisons follow the
	// provider's semantics.
	Raw string
	LTS bool
	// Group optionally names a cluster ListRemote's result belongs to
	// (e.g. a JDK vendor: "corretto", "temurin"), for providers whose
	// remote listing is naturally organized in more than one series.
	// Empty for providers with a single series; the CLI prints
	// grouped entries nested under their Group instead of as one flat
	// list.
	Group string
}

// Artifact is a completed, verified download ready to install.
type Artifact struct {
	Path     string // file in the cache
	Checksum string // verified sha256
}

// Provider is the interface every tool (java, node, go, python...)
// implements. The core knows nothing about specific tools.
type Provider interface {
	Name() string

	// ListRemote lists the versions available at the official source.
	ListRemote(ctx context.Context) ([]Version, error)

	// Resolve turns a user spec ("21", "lts", "latest", "^3.12")
	// into a concrete version.
	Resolve(ctx context.Context, spec string) (Version, error)

	// Download fetches and verifies the artifact into the cache.
	Download(ctx context.Context, v Version, cacheDir string) (Artifact, error)

	// Install extracts/installs the artifact into dir
	// (tools/<tool>/<version>).
	Install(a Artifact, dir string) error

	// EnvVars returns variables the tool requires pointing at dir
	// (JAVA_HOME, GOROOT...). May be empty.
	EnvVars(dir string) map[string]string

	// Executables lists the binaries that get a shim
	// (e.g. node and npm; java, javac and jar).
	Executables() []string
}
