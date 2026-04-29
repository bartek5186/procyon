package main

import (
	"flag"
	"log"
)

const appVersion = "0.1.0"

var (
	migrate    bool
	configPath string
)

func init() {
	flag.BoolVar(&migrate, "migrate", false, "Run DB migrations on start")
	flag.StringVar(&configPath, "config", "", "Path to runtime config file")
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
