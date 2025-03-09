package main

import (
	"fmt"
	"github.com/MrPuls/local-ci/cmd/local-ci/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
