package main

import (
	"fmt"
	"local-ci/cmd/config"
)

func main() {
	yamlConf := config.Config{}
	err := yamlConf.GetConfig("foo.yaml")
	if err != nil {
		panic(err)
	}
	errVal := config.ValidateConfig(yamlConf)
	if errVal != nil {
		panic(errVal)
	}
	for item := range yamlConf.Blocks {
		fmt.Println(item)
		fmt.Println(yamlConf.Blocks[item])
	}

	//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	//defer cancel()
	//docker.ExecuteConfigPipeline(yamlConf, ctx)

}
