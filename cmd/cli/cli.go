package cli

import (
	"flag"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/config"
	"github.com/MrPuls/local-ci/cmd/docker"
	"os"
)

func Start(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	filePath := fs.String(
		"f", ".local-ci/local.yaml",
		"path to config file (defaults to .local-ci/local.yaml)",
	)
	fs.String("help", "", "\"start\" - to start the pipeline")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s", err)
		return
	}
	fmt.Printf("Getting config from file: %v", *filePath)
	pwd, _ := os.Getwd()
	fmt.Printf("Working dir: %s\n", pwd)
	yamlConf := config.Config{}
	err := yamlConf.GetConfig(pwd + "/" + *filePath)
	if err != nil {
		panic(err)
	}
	errVal := config.ValidateConfig(yamlConf)
	if errVal != nil {
		panic(errVal)
	}
	for item := range yamlConf.Blocks {
		docker.ExecuteConfigPipeline(yamlConf.Blocks[item])
	}
}
