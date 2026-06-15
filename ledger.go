package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ledgerLine renders one operation as a single audit record: a timestamp
// followed by the verb, kind, name, and any detail. It is printed to stdout, one
// line per operation; redirect stdout to keep a ledger.
func ledgerLine(e *ledgerEntry) string {
	line := fmt.Sprintf("%s %-6s %-7s %s", time.Now().Format(time.RFC3339), e.verb, e.kind, e.name)
	if e.detail != "" {
		line += " " + e.detail
	}
	return line
}

// kvValue is the generated value for a churned key: deterministic content that
// changes on each rewrite (the embedded rev makes a rewrite visible).
func kvValue(key string, rev int) []byte {
	return []byte(fmt.Sprintf("key=%s\nrev=%d\nts=%s\n", key, rev, time.Now().Format(time.RFC3339)))
}

// parseRev reads the rev line out of a generated KV value; absent or malformed
// means start the counter from zero.
func parseRev(text string) int {
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "rev=") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(line, "rev="))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func splitInstance(s string) (svc, node string) {
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func short(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}
