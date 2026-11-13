package main

import (
	"context"
	"fmt"
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

func update(force bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current := versionString()
	installed, err := selfupdate.Update(ctx, selfupdate.LatestURL, current, executable(), runtime.GOOS, runtime.GOARCH, force)
	if err != nil {
		fail(err.Error())
	}
	if installed == "" {
		fmt.Printf("wordle-solver %s is the latest version.\n", current)
		return
	}
	fmt.Printf("Updated wordle-solver %s to %s.\n", current, installed)
}
