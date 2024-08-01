package util

import "fmt"

var (
	majorVersion = 1 // PLATO major version here
	minorVersion = 1 // PLATO minor version here
	// BuildID is the default builder ID
	BuildID = "development" // Default value for development builds
)

// VersionInfo holds the version information
var VersionInfo = struct {
	MajorVersion int    // PLATO major version here
	MinorVersion int    // PLATO minor version here
	BuildID      string // Build ID
}{
	MajorVersion: majorVersion,
	MinorVersion: minorVersion,
	BuildID:      BuildID,
}

// Version returns the version string for this build
func Version() string {
	return fmt.Sprintf("%d.%d-%s", VersionInfo.MajorVersion, VersionInfo.MinorVersion, BuildID)
}

// VersionMajorMinor returns only the Major.Minor version number
func VersionMajorMinor() string {
	return fmt.Sprintf("%d.%d", VersionInfo.MajorVersion, VersionInfo.MinorVersion)
}
