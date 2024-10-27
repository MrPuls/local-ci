package main

import (
	"flag"
	"fmt"
	"local-ci/cmd/cli"
)

func main() {
	flag.Parse()
	fmt.Println(*cli.Foo)
	//pwd, _ := os.Getwd()
	//fmt.Printf("Working dir: %s\n", pwd)
	//yamlConf := config.Config{}
	//err := yamlConf.GetConfig(*cli.FilePath)
	//if err != nil {
	//	panic(err)
	//}
	//errVal := config.ValidateConfig(yamlConf)
	//if errVal != nil {
	//	panic(errVal)
	//}
	//for item := range yamlConf.Blocks {
	//	fmt.Println(item)
	//	fmt.Println(yamlConf.Blocks[item].Variables["FOO"])
	//	fmt.Println(yamlConf.Blocks[item])
	//	docker.ExecuteConfigPipeline(yamlConf.Blocks[item])
	//}

}
