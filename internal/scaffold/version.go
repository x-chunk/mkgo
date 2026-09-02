package scaffold

import "runtime"

// runtimeVersion is a seam that lets tests pin the Go version.
var runtimeVersion = runtime.Version
