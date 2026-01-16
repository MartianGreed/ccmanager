package main

import (
	"fmt"
	"os"

	"github.com/valentindosimont/ccmanager/internal/app"
)

func main() {
	cfg, fileCfg := app.LoadConfig()

	application, err := app.New(cfg, fileCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}
	defer application.Close()

	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
