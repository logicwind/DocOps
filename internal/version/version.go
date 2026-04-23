package version

import (
	"fmt"
	"runtime/debug"
)

// Populated at release time via -ldflags by goreleaser.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	v := Version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	return fmt.Sprintf("docops %s (commit %s, built %s)\nan open-source project by Logicwind · https://github.com/logicwind/DocOps", v, Commit, Date)
}
