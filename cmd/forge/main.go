package main

import (
	"fmt"
	"github.com/iandrummond-socal/forge/internal/cli"
	"os"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
