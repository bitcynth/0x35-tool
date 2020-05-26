package main

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
