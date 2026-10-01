//go:build !windows

package progress

import "io"

func enableVT(io.Writer) bool { return true }
