package cli

import (
	"flag"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/config"
	"github.com/MrPuls/local-ci/cmd/docker"
	"os"
)

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s", err)
		return
	}
	fmt.Println("Starting pipeline ...")

	pwd, _ := os.Getwd()
	fmt.Printf("Getting config from file: %s/.local-ci.yaml", pwd)
	yamlConf := config.Config{}
	err := yamlConf.GetConfig(pwd + "/.local-ci.yaml")
	if err != nil {
		panic(err)
	}
	errVal := config.ValidateConfig(yamlConf)
	if errVal != nil {
		panic(errVal)
	}
	for item := range yamlConf.Blocks {
		docker.ExecuteConfigPipeline(pwd, yamlConf.Blocks[item])
	}
}

func Execute(args []string) {
	switch args[1] {
	case "run":
		run(args)
	}
}
