package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
