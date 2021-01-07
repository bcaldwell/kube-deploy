package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bcaldwell/kube-deploy/pkg/deploy"
	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
)

func main() {
	d := deploy.Deploy{}

	flag.StringVar(&d.ConfigFolder, "configFolder", "", "")
	flag.Parse()

	if d.ConfigFolder == "" {
		fmt.Println("--configFolder is required")
		return
	}

	err := d.Run()
	if err != nil {
		logger.Log("Error deploying %s", err)
		os.Exit(1)
	}
}
