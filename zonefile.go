package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// The YAML header of a 0x35 zone file
type zoneFileHeader struct {
	Name      string            `yaml:"zone"`
	TTL       int               `yaml:"ttl"`  // the TTL of the zone, also a special variable
	Variables map[string]string `yaml:"vars"` // variables that can be referenced in the zone file body
}

type zoneFileEntry struct {
	Name      string
	Type      string
	Data      string
	TTL       int
	ExtraData zoneFileExtraData
}

// Reserved for future use, for potential features such as comments or whatever
type zoneFileExtraData struct {
}

// Entry with ExtraData and TTL specified
var regexZoneEntryWithExtraTTL = regexp.MustCompile(`(?i)^([@a-z0-9\-_\.\*]+)\s+([0-9]+)\s+([A-Z]+)\s+(.+)\s+({.*})$`)

// Entry with ExtraData specified but not TTL
var regexZoneEntryWithExtra = regexp.MustCompile(`(?i)^([@a-z0-9\-_\.\*]+)\s+([A-Z]+)\s+(.+)\s+({.*})$`)

// Entry with TTL specified but not ExtraData
var regexZoneEntryWithTTL = regexp.MustCompile(`(?i)^([@a-z0-9\-_\.\*]+)\s+([0-9]+)\s+([A-Z]+)\s+(.+)$`)

// Entry with neither TTL or ExtraData specified
var regexZoneEntry = regexp.MustCompile(`(?i)^([@a-z0-9\-_\.\*]+)\s+([A-Z]+)\s+(.+)$`)

// Regex to extract a variable from the zone file body
var regexZoneVariable = regexp.MustCompile(`%\(([a-zA-Z0-9\-_]+)\)%`)

func parseZoneFile(zoneFile string) (zoneFileHeader, []zoneFileEntry, error) {
	var header zoneFileHeader
	var entries []zoneFileEntry

	// Read in the zone file and "clean" it
	zoneFileBytes, err := ioutil.ReadFile(zoneFile)
	if err != nil {
		return header, nil, fmt.Errorf("failed to read zone file: %v", err)
	}
	zoneFileStr := string(zoneFileBytes)
	zoneFileStr = strings.ReplaceAll(zoneFileStr, "\r", "")

	// Split the header from the body
	zoneFileParts := strings.SplitN(zoneFileStr, "\n---\n", 2)
	if len(zoneFileParts) != 2 {
		return header, nil, fmt.Errorf("not a valid zone file")
	}
	zoneFileHeader := zoneFileParts[0]
	zoneFileBody := zoneFileParts[1]

	// Parse the header
	err = yaml.Unmarshal([]byte(zoneFileHeader), &header)
	if err != nil {
		return header, nil, fmt.Errorf("invalid zone file header")
	}

	// Zone variables
	zoneFileBody = regexZoneVariable.ReplaceAllStringFunc(zoneFileBody, func(in string) string {
		variableName := in[2 : len(in)-2]

		if variableName == "ttl" {
			return strconv.Itoa(header.TTL)
		}

		if variableName == "zone" {
			return header.Name
		}

		// Check if the variable exists in the custom variables
		variableValue, ok := header.Variables[variableName]
		if ok {
			return variableValue
		}

		return in
	})

	lines := strings.Split(zoneFileBody, "\n")
	if len(lines) < 1 {
		return header, nil, fmt.Errorf("no entries in the zone file")
	}

	// Loop through all the lines and parse the entries
	for i, line := range lines {
		// Test if line is a comment
		testLine := strings.TrimSpace(line)
		if len(testLine) == 0 || testLine[0] == '#' {
			// it is a comment
			continue
		}

		entry, err := parseZoneFileEntry(line)
		if err != nil {
			return header, nil, fmt.Errorf("line %d: %v", i+1, err)
		}

		// Expand the entry name
		entry.Name = expandEntryName(header.Name, entry.Name)

		entries = append(entries, entry)
	}

	return header, entries, nil
}

func parseZoneFileEntry(line string) (zoneFileEntry, error) {
	var entry zoneFileEntry

	matchEntryExtraTTL := regexZoneEntryWithExtraTTL.MatchString(line)
	matchEntryExtra := regexZoneEntryWithExtra.MatchString(line)
	matchEntryTTL := regexZoneEntryWithTTL.MatchString(line)
	matchEntry := regexZoneEntry.MatchString(line)

	// There is a decent amount of duplicate code here, could probably be improved.
	if matchEntryExtraTTL {
		submatch := regexZoneEntryWithExtraTTL.FindStringSubmatch(line)

		ttl, err := strconv.Atoi(submatch[2])
		if err != nil {
			return entry, fmt.Errorf("invalid TTL value: %v", err)
		}

		entry = zoneFileEntry{
			Name: submatch[1],
			TTL:  ttl,
			Type: submatch[3],
			Data: submatch[4],
		}

		// Parse extra data
		err = json.Unmarshal([]byte(submatch[5]), &entry.ExtraData)
		if err != nil {
			return entry, fmt.Errorf("invalid extra data: %v", err)
		}
	} else if matchEntryExtra {
		submatch := regexZoneEntryWithExtra.FindStringSubmatch(line)

		entry = zoneFileEntry{
			Name: submatch[1],
			TTL:  -1, // -1 = un-specified
			Type: submatch[2],
			Data: submatch[3],
		}

		// Parse extra data
		err := json.Unmarshal([]byte(submatch[5]), &entry.ExtraData)
		if err != nil {
			return entry, fmt.Errorf("invalid extra data: %v", err)
		}
	} else if matchEntryTTL {
		submatch := regexZoneEntryWithTTL.FindStringSubmatch(line)

		ttl, err := strconv.Atoi(submatch[2])
		if err != nil {
			return entry, fmt.Errorf("invalid TTL value: %v", err)
		}

		entry = zoneFileEntry{
			Name: submatch[1],
			TTL:  ttl,
			Type: submatch[3],
			Data: submatch[4],
		}
	} else if matchEntry {
		submatch := regexZoneEntry.FindStringSubmatch(line)

		entry = zoneFileEntry{
			Name: submatch[1],
			TTL:  -1, // -1 = un-specified
			Type: submatch[2],
			Data: submatch[3],
		}
	} else {
		return entry, fmt.Errorf("invalid entry")
	}

	return entry, nil
}
