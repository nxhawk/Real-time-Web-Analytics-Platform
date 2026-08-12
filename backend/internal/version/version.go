// Package version exposes build metadata injected at link time.
//
// Values are set with -ldflags, for example:
//
//	go build -ldflags "-X github.com/nxhawk/pulse-analytics/backend/internal/version.Commit=$(git rev-parse HEAD)"
//
// When the binary is built without those flags (go run, tests) the values stay "unknown",
// which is exactly what /version should report in that situation.
package version

import "runtime"

var (
	// Commit is the git SHA the binary was built from.
	Commit = "unknown"
	// BuildTime is the RFC3339 timestamp of the build.
	BuildTime = "unknown"
	// Tag is the release tag, when the build came from one.
	Tag = "dev"
)

// Info is the payload returned by GET /version.
type Info struct {
	Tag       string `json:"tag"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// Get returns the build metadata of the running binary.
func Get() Info {
	return Info{
		Tag:       Tag,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
	}
}
