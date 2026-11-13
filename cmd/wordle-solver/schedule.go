package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/notzenco/wordle-solver/internal/schedule"
)

// scheduleCmd sets up (or with remove, takes down) the daily run.
func scheduleCmd(at string, remove bool) {
	if remove {
		if err := schedule.Remove(); err != nil {
			fail(err.Error())
		}
		fmt.Println("Daily run removed.")
		return
	}
	var hour, minute int
	if _, err := fmt.Sscanf(at, "%d:%d", &hour, &minute); err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		fail("-at must look like 00:05")
	}
	exe := executable()
	env := map[string]string{}
	if d := os.Getenv("WORDLE_SOLVER_HOME"); d != "" {
		d, err := filepath.Abs(d)
		if err != nil {
			fail(err.Error())
		}
		env["WORDLE_SOLVER_HOME"] = d
	}
	log, err := schedule.Install(exe, hour, minute, env)
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("Daily run installed: %s plays at %02d:%02d local time and catches up on missed days.\n", filepath.Base(exe), hour, minute)
	if runtime.GOOS == "linux" {
		fmt.Println("It runs while you are logged in; 'loginctl enable-linger' lets it run when you are not.")
	}
	fmt.Printf("Log: %s\nRemove with: wordle-solver schedule -remove\n", log)
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
