package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrPuls/local-ci/cmd/local-ci/cli"
)

func main() {

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	if err := cli.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	done <- true

	select {
	case <-done:
	case sig := <-sigChan:
		//TODO: here is where the cleanup should start.
		//	Somehow need to propagate the command to stop executor, remove all running containers and cancel all contexts.
		//	Perhaps the pipeline should run in goroutine and then if signal comes - stop the execution,
		//	if there is a container - send id to a cleanup func through channels
		fmt.Println(sig)

	}
}
