package util

var (
	majorVersion = "1"           // PLATO major version here
	minorVersion = "0"           // PLATO minor version here
	buildID      = "development" // Default value for development builds - we use Go's loader flags to change at link time
)

// Version returns the version string for this build
func Version() string {
	return majorVersion + "." + minorVersion + " " + buildID
}
