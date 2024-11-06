package main

import (
	"fmt"
	"github.com/MrPuls/local-ci/cmd/cli"
	"os"
)

func main() {
	fmt.Println(os.Args[2:])
	if os.Args[1] == "start" {
		cli.Start(os.Args[2:])
	}
}
