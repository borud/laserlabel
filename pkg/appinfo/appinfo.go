// Package appinfo holds build information injected via linker flags.
package appinfo

// Build information, set with -ldflags at build time.
var (
	CommitHash    = "none"
	BuildTime     = "unknown"
	TaggedVersion = "unknown"
	BuiltBy       = "unknown"
)
