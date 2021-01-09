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
	target := ""

	flag.StringVar(&d.ConfigFolder, "configFolder", "", "")
	flag.StringVar(&target, "target", "", "")
	flag.Parse()

	if d.ConfigFolder == "" {
		fmt.Println("--configFolder is required")
		return
	}

	err := d.Run(target)
	if err != nil {
		logger.Log("Error deploying %s", err)
		os.Exit(1)
	}
}
