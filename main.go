package main

import (
	_ "embed"
	"fmt"
	"os"

	"silo/pkg/cli"
)

//go:embed config/silo.toml
var projectConfig []byte

func main() {
	cli.SetProjectConfig(projectConfig)
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
