package main

import (
	"flag"
	"fmt"
)

var configFile = flag.String("config", "$HOME/.0x35-tool.json", "the path to the config file")
var syncFlag = flag.Bool("sync", false, "update the 0x35 servers with the zone file(s) provided")

func main() {
	flag.Parse()

	loadConfig()

	if *syncFlag {
		// In the case of sync, all non-flag args are zone files
		for _, zoneFile := range flag.Args() {
			fmt.Printf("Updating from file: %s\n", zoneFile)
		}
	}
}
