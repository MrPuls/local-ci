package main

import (
	"fmt"
	"github.com/MrPuls/local-ci/cmd/cli"
	"os"
)

func main() {
	fmt.Println(os.Args[2:])
	// TODO: needs to be more informative and be able to run with commands other than start. Probably should be something like cli.Execute and then inside we parse os args and act accordingly
	if os.Args[1] == "start" {
		cli.Start(os.Args[2:])
	}
}
