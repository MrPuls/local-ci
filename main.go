package main

import (
	"flag"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/cli"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Usage: local-ci [command]")
		fmt.Println("Commands:")
		fmt.Println("  run       Run the pipeline")
		fmt.Println("  --version Show current version")
		os.Exit(0)
	}
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(0)
	}

	cli.Execute(os.Args)
}
