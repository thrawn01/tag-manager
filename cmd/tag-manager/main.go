package main

import (
	"fmt"
	"os"

	tagmanager "github.com/thrawn01/tag-manager"
)

func main() {
	if err := tagmanager.RunCmd(os.Args, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
