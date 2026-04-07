package version

var (
	// Version is populated during the build process via -ldflags.
	Version = "dev"
	// Commit is populated during the build process via -ldflags.
	Commit = "unknown"
	// BuildDate is populated during the build process via -ldflags.
	BuildDate = "unknown"
)
