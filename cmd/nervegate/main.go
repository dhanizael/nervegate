package main

import (
	"github.com/hxmdxnx/nervegate/cmd/nervegate/commands"
)

var (
	Version   = "0.1.0-alpha"
	Commit    = "dev"
	BuildDate = "unknown"
)

func main() {
	commands.Execute(Version, Commit, BuildDate)
}
