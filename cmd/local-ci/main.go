package main

import (
	"fmt"
	"os"

	"github.com/MrPuls/local-ci/cmd/local-ci/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
