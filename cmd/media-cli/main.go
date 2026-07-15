package main

import (
	"os"
	"runtime/debug"

	"radioplatform-media-ci/internal/buildinfo"
	"radioplatform-media-ci/internal/cli"
)

func main() {
	if info, ok := debug.ReadBuildInfo(); ok {
		buildinfo.GoVersion = info.GoVersion
	}

	rootCmd := cli.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
