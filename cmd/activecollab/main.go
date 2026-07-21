package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/microHoffman/activecollab-cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(resolvedVersion()))
}

func resolvedVersion() string {
	info, ok := debug.ReadBuildInfo()
	return selectVersion(version, info, ok)
}

func selectVersion(injected string, info *debug.BuildInfo, ok bool) string {
	if injected != "" && injected != "dev" {
		return strings.TrimPrefix(injected, "v")
	}
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}
