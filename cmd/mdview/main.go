package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kyaoi/mdview/internal/app"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mdview <path-to-markdown-or-directory>")
		os.Exit(1)
	}

	target := filepath.Clean(os.Args[1])
	if err := app.Run(target); err != nil {
		log.Fatal(err)
	}
}
