package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

// The type of the config file, containing the possible settings.
type configType struct {
	BaseURL  string `json:"base_url"`
	ServerID string `json:"server_id"`
	APIKey   string `json:"api_key"`
}

var config configType

// Load and parse the config file
// Also set the default values
func loadConfig() {
	config = configType{}
	config.ServerID = "localhost" // "localhost" is the default server ID

	// Since $HOME is used in the default value, we need to expand env vars
	configPath := os.ExpandEnv(*configFile)

	configJSON, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("uh oh, I failed to read the config file: %v", err)
	}

	err = json.Unmarshal(configJSON, &config)
	if err != nil {
		log.Fatalf("uh oh, I failed to parse the config json: %v", err)
	}

	// Check for required options
	if config.BaseURL == "" {
		log.Fatal("the base url is a required setting in the config file")
	}
	if config.APIKey == "" {
		log.Fatal("the API key is a required setting in the config file")
	}
}
