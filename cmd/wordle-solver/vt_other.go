//go:build !windows

package main

import "os"

// enableVT reports whether a terminal shows ANSI colour; outside Windows it
// always does.
func enableVT(*os.File) bool { return true }
