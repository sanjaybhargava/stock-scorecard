// Package clientid extracts the Zerodha client ID from tradebook CSV filenames.
// Zerodha client IDs are two letters followed by digits (e.g. BT2632, XY1234).
// The expected filename pattern is {clientID}_*.csv, e.g. BT2632_20200101_20201231.csv.
package clientid

import (
	"fmt"
	"regexp"
)

// clientIDRe matches a Zerodha client ID at the start of a filename:
// 2-3 uppercase letters followed by digits, then underscore.
// Covers formats like BT2632, ZY7393, DUA527, etc.
var clientIDRe = regexp.MustCompile(`^([A-Z]{2,3}\d+)_`)

// Extract parses the client ID from a list of filenames (basenames).
// Returns the client ID if exactly one is found. Returns an error if
// no client ID is found or if multiple different IDs are detected.
func Extract(filenames []string) (string, error) {
	seen := make(map[string]bool)
	for _, name := range filenames {
		m := clientIDRe.FindStringSubmatch(name)
		if len(m) >= 2 {
			seen[m[1]] = true
		}
	}

	if len(seen) == 0 {
		return "", fmt.Errorf("no client ID found in filenames (expected {clientID}_*.csv, e.g. BT2632_20200101_20201231.csv)")
	}

	// Collect unique IDs
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}

	if len(ids) > 1 {
		return "", fmt.Errorf("multiple client IDs found: %v (expected one client per directory)", ids)
	}

	return ids[0], nil
}
