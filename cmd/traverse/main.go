package main

import (
	"github.com/funkymonkeymonk/traverse/cmd/traverse/cli"
)

// Build information - set during build
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set version info in the cli package
	cli.Version = version
	cli.Commit = commit
	cli.BuildDate = date

	// Execute the CLI
	cli.Execute()
}
