package main

import (
	"flag"
	"fmt"

	"github.com/bcaldwell/kube-deploy/pkg/deploy"
)

func main() {
	d := deploy.Deploy{}

	flag.StringVar(&d.ConfigFolder, "configFolder", "", "")
	flag.Parse()

	if d.ConfigFolder == "" {
		fmt.Println("--configFolder is required")
		return
	}

	fmt.Print(d.Run())
}
