package main

import (
	"flag"
	"fmt"
	"os"
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

			header, entries, err := parseZoneFile(zoneFile)
			if err != nil {
				fmt.Printf("failed to parse file: %v\n", err)
				os.Exit(1)
			}

			// Fetch the existing zone from PDNS
			pdnsZone, err := pdnsGetZone(header.Name)
			if err == errNoSuchZone {
				fmt.Printf("no such zone, treating as new zone\n")
			} else if err != nil {
				fmt.Printf("failed to get zone from server: %v\n", err)
			}

			// List the changes
			rrSetChanges, err := pdnsGetZoneChanges(pdnsZone, header, entries)
			if err == errAlreadyUpToDate {
				fmt.Printf("Zone already up to date!\n")
				continue
			}
			changes := formatChangesList(rrSetChanges, pdnsZone)
			for _, change := range changes {
				fmt.Printf("%s\n", change)
			}
		}
	}
}
