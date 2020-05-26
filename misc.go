package main

import "fmt"

// Helper function to expand names
// Example 1: cat -> cat.example.com.
// Example 2: @ -> example.com.
// Example 3: example.com. -> example.com.
func expandEntryName(zoneName string, name string) string {
	if name == "@" {
		return zoneName
	}

	// if the last character is '.', it is a FQDN
	if name[len(name)-1] == '.' {
		return name
	}

	// Append zoneName to name
	return fmt.Sprintf("%s.%s", name, zoneName)
}
