package network

import "strings"

// SplitAddressList splits a comma-separated list of IP addresses or CIDR ranges. Each entry is trimmed and blank
// entries are dropped. It returns nil when the list holds no entries.
func SplitAddressList(list string) []string {
	var entries []string
	for _, entry := range strings.Split(list, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}
