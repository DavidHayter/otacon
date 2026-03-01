package main

import (
	"fmt"
	"os"

	"github.com/merthan/otacon/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	buildInfo := cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	rootCmd := cli.NewRootCommand(buildInfo)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
