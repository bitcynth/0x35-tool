package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
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
			isNewZone := false
			if err == errNoSuchZone {
				isNewZone = true
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
			fmt.Println()

			// Ask if the user wants to proceed
			cliReader := bufio.NewReader(os.Stdin)
			fmt.Print("Do you want to update (type \"yes\" and hit enter): ")
			text, _ := cliReader.ReadString('\n')
			text = strings.ReplaceAll(text, "\r", "")
			if text != "yes\n" {
				continue
			}

			// Actually update the zone
			err = pdnsUpdateZone(rrSetChanges, isNewZone, header)
			if err != nil {
				fmt.Printf("failed to update zone: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Zone (%s) updated!\n", header.Name)
		}
	}
}
