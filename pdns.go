package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Represents a zone to be passed to the PowerDNS API
type pdnsZone struct {
	RRSets []pdnsRRSet `json:"rrsets"`
	Serial int         `json:"serial"`
	Name   string      `json:"name"`
	Kind   string      `json:"kind"`
}

type pdnsRRSet struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	TTL        int           `json:"ttl"`
	ChangeType string        `json:"changetype"`
	Records    []pdnsRecord  `json:"records"`
	Comments   []pdnsComment `json:"comments"`
}

type pdnsRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
	SetPTR   bool   `json:"set-ptr"` // Not used in 0x35 currently, used for reverse DNS entries
}

type pdnsComment struct {
	Content    string `json:"content"`
	Account    string `json:"account"`
	ModifiedAt int64  `json:"modified_at"`
}

var errNoSuchZone = errors.New("the zone doesn't exist")
var errAlreadyUpToDate = errors.New("the zone is already up to date")

func pdnsCallAPI(method string, url string, payload interface{}) (*http.Response, error) {
	var body io.Reader

	// Populate body if there is a payload
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal JSON error: %v", err)
		}
		body = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error initiating HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-key", config.APIKey) // Global PDNS API key

	// Actually execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing call: %v", err)
	}

	return resp, nil
}

// Fetches a zone with the PowerDNS API and parses it
func pdnsGetZone(zoneName string) (pdnsZone, error) {
	url := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s", config.BaseURL, config.ServerID, zoneName)

	var zone pdnsZone

	resp, err := pdnsCallAPI("GET", url, nil)
	if err != nil {
		return zone, fmt.Errorf("error calling API: %v", err)
	}

	if resp.StatusCode == 404 {
		return zone, errNoSuchZone
	}

	err = json.NewDecoder(resp.Body).Decode(&zone)
	if err != nil {
		return zone, fmt.Errorf("error decoding JSON: %v", err)
	}

	return zone, nil
}

// This method checks for changes that need to be made to the server zone.
// This function is quite massive and could probably be a lot cleaner.
func pdnsGetZoneChanges(initialZone pdnsZone, header zoneFileHeader, entries []zoneFileEntry) (map[string]pdnsRRSet, error) {
	// Entries are grouped by record type and name, example: "@-ALIAS"
	rrSets := make(map[string]pdnsRRSet)
	for _, entry := range entries {
		rrSetName := entry.Name + "-" + entry.Type
		ttl := entry.TTL

		// if TTL is unspecified (-1), use the header TTL value
		if ttl == -1 {
			ttl = header.TTL
		}

		rrSet, ok := rrSets[rrSetName]
		// Check if the RRSet (group of entries) already exists in the map
		// and if not, create it.
		if !ok {
			rrSet = pdnsRRSet{
				Name: entry.Name,
				Type: entry.Type,
				TTL:  ttl,
			}
		}

		// Add the PNS record to the RRSet
		record := pdnsRecord{
			Content: entry.Data,
		}
		rrSet.Records = append(rrSet.Records, record)
		rrSets[rrSetName] = rrSet
	}

	// Check that there is a single SOA record in the zone
	if len(rrSets[header.Name+"-SOA"].Records) != 1 {
		return nil, fmt.Errorf("you need exactly 1 SOA record")
	}

	// Check if SERIAL_INC is being used and store it.
	// This is a bit of a hack to avoid serial changes being detected as a change.
	soaContent := rrSets[header.Name+"-SOA"].Records[0].Content
	usingSerialInc := false
	rrSets[header.Name+"-SOA"].Records[0].Content = regexZoneVariable.ReplaceAllStringFunc(soaContent, func(in string) string {
		if in[2:len(in)-2] == "SERIAL_INC" {
			usingSerialInc = true
			return strconv.Itoa(initialZone.Serial)
		}
		return in
	})

	rrSetChanges := make(map[string]pdnsRRSet)
	rrSetInitialValues := make(map[string]pdnsRRSet)

	// Loop through the initial RRSets and check what needs to be changed.
	for _, rrSet := range initialZone.RRSets {
		rrSetGroup := rrSet.Name + "-" + rrSet.Type
		rrSetInitialValues[rrSetGroup] = rrSet // Store the initial state

		// Check if the RRSet exists in the new state.
		// If it doesn't exist, set change type to delete.
		rrSetChange, ok := rrSets[rrSetGroup]
		if !ok {
			rrSetChange = rrSet
			rrSetChange.ChangeType = "DELETE"
			rrSetChanges[rrSetGroup] = rrSetChange
		}

		if rrSet.TTL != rrSetChange.TTL || len(rrSet.Records) != len(rrSetChange.Records) {
			rrSetChange.ChangeType = "REPLACE"
		}

		for _, record := range rrSet.Records {
			for i, changedRecord := range rrSetChange.Records {
				if record == changedRecord {
					// We found a record in the new state that matches the old state
					break
				}

				if len(rrSetChange.Records) == i+1 {
					rrSetChange.ChangeType = "REPLACE"
				}
			}
		}

		rrSetChanges[rrSetGroup] = rrSetChange
	}

	// Loop through new state and check the change type in the changes.
	for _, rrSet := range rrSets {
		rrSetGroup := rrSet.Name + "-" + rrSet.Type
		rrSetChange, ok := rrSetChanges[rrSetGroup]
		if ok {
			if rrSetChange.ChangeType == "" {
				// If the RRSet is already up to date, delete the change.
				delete(rrSetChanges, rrSetGroup)
			}
			continue
		}
		rrSet.ChangeType = "ADD" // This is a temporary status for this tool, will be changed to REPLACE later.
		rrSetChanges[rrSetGroup] = rrSet
	}

	// If there are no changes, return
	if len(rrSetChanges) == 0 {
		return nil, errAlreadyUpToDate
	}

	// Update the SERIAL_INC value and update the SOA record
	if usingSerialInc {
		soaRRSet := rrSets[header.Name+"-SOA"]
		rrSets[header.Name+"-SOA"].Records[0].Content = regexZoneVariable.ReplaceAllStringFunc(soaContent, func(in string) string {
			if in[2:len(in)-2] == "SERIAL_INC" {
				return strconv.Itoa(initialZone.Serial + 1)
			}
			return in
		})
		soaRRSet.ChangeType = "REPLACE"
		rrSetChanges[header.Name+"-SOA"] = soaRRSet
	}

	return rrSetChanges, nil
}

// This executes and update of the zone from the RRSet changes supplied.
func pdnsUpdateZone(rrSetChanges map[string]pdnsRRSet, isNewZone bool, header zoneFileHeader) error {
	payload := pdnsZone{}

	url := fmt.Sprintf("%s/api/v1/servers/%s/zones", config.BaseURL, config.ServerID)
	if !isNewZone {
		url = fmt.Sprintf("%s/%s", url, header.Name)
	}

	// Loop through the RRSet changes and add them to the payload.
	for _, rrSet := range rrSetChanges {
		if rrSet.ChangeType == "ADD" {
			// ADD is not a real change type, but it helps with the format changes part.
			rrSet.ChangeType = "REPLACE"
		} else if rrSet.ChangeType == "DELETE" {
			// DELETE doesn't want any records or comments
			rrSet.Records = []pdnsRecord{}
			rrSet.Comments = []pdnsComment{}
		}
		payload.RRSets = append(payload.RRSets, rrSet)
	}

	if isNewZone {
		payload.Name = header.Name
		payload.Kind = "Native"
		resp, err := pdnsCallAPI("POST", url, payload)
		if err != nil {
			return fmt.Errorf("error creating zone: %v", err)
		}
		if resp.StatusCode != 201 {
			return fmt.Errorf("error creating zone: %s", resp.Status)
		}
	} else {
		resp, err := pdnsCallAPI("PATCH", url, payload)
		if err != nil {
			return fmt.Errorf("error updating zone: %v", err)
		}
		if resp.StatusCode != 204 {
			return fmt.Errorf("error updating zone: %s", resp.Status)
		}
	}

	return nil
}

// Produces a list of zone changes to visualize the changes.
func formatChangesList(rrSetChanges map[string]pdnsRRSet, initialZone pdnsZone) []string {
	// Sort the initial RRSets
	initialRRSets := make(map[string]pdnsRRSet)
	for _, rrSet := range initialZone.RRSets {
		rrSetGroup := rrSet.Name + "-" + rrSet.Type
		initialRRSets[rrSetGroup] = rrSet
	}

	lines := []string{}
	for rrSetGroup, rrSet := range rrSetChanges {
		initialRRSet := initialRRSets[rrSetGroup]

		if rrSet.ChangeType == "ADD" {
			for _, record := range rrSet.Records {
				line := fmt.Sprintf("+ %s %d %s %s", rrSet.Name, rrSet.TTL, rrSet.Type, record.Content)
				lines = append(lines, line)
			}
		} else if rrSet.ChangeType == "DELETE" {
			for _, record := range rrSet.Records {
				line := fmt.Sprintf("- %s %d %s %s", rrSet.Name, rrSet.TTL, rrSet.Type, record.Content)
				lines = append(lines, line)
			}
		} else if rrSet.ChangeType == "REPLACE" {
			for _, record := range rrSet.Records {
				line := fmt.Sprintf("+ %s %d %s %s", rrSet.Name, rrSet.TTL, rrSet.Type, record.Content)
				lines = append(lines, line)
			}

			for _, record := range initialRRSet.Records {
				line := fmt.Sprintf("- %s %d %s %s", initialRRSet.Name, initialRRSet.TTL, initialRRSet.Type, record.Content)
				lines = append(lines, line)
			}
		}
	}

	return lines
}
