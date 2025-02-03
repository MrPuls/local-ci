package cli

import (
	"flag"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/config"
	"github.com/MrPuls/local-ci/cmd/docker"
	"os"
)

func Execute(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.String("help", "", "\"run\" - to start the pipeline")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s", err)
		return
	}

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
