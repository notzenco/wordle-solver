package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/notzenco/wordle-solver/internal/selfupdate"
)

// version is set at release build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func versionString() string {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return version
}

// latestURL is the release endpoint; tests point it at a local server.
var latestURL = selfupdate.LatestURL

// checkUpdate says whether a newer release exists, without installing it.
func checkUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current := versionString()
	tag, newer, isRelease, err := selfupdate.Check(ctx, latestURL, current)
	if err != nil {
		fail(err.Error())
	}
	switch {
	case !isRelease:
		fmt.Fprintf(stdout, "This is a development build (%s); the latest release is %s.\n", current, tag)
	case newer:
		fmt.Fprintf(stdout, "%s is available (you have %s); 'wordle-solver update' installs it.\n", tag, current)
	default:
		fmt.Fprintf(stdout, "wordle-solver %s is the latest version.\n", current)
	}
}

func update(force bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current := versionString()
	installed, err := selfupdate.Update(ctx, latestURL, current, executable(), runtime.GOOS, runtime.GOARCH, force)
	if err != nil {
		fail(err.Error())
	}
	if installed == "" {
		fmt.Fprintf(stdout, "wordle-solver %s is the latest version.\n", current)
		return
	}
	fmt.Fprintf(stdout, "Updated wordle-solver %s to %s.\n", current, installed)
}

// executable is the path of the running binary, with symlinks resolved.
func executable() string {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fail(err.Error())
	}
	return exe
}
