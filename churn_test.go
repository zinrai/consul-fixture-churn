package main

import (
	"errors"
	"strings"
	"testing"
)

// dcOf builds a datacenter lookup from a map; an address absent from the map is
// treated as unreachable.
func dcOf(byAddr map[string]string) func(string) (string, error) {
	return func(addr string) (string, error) {
		if dc, ok := byAddr[addr]; ok {
			return dc, nil
		}
		return "", errors.New("unreachable")
	}
}

func TestSelectHost(t *testing.T) {
	const want = "dc-verify"

	// A reachable host in the declared datacenter is chosen.
	if addr, err := selectHost([]string{"h1", "h2"}, want, dcOf(map[string]string{"h1": want, "h2": want})); err != nil || addr != "h1" {
		t.Fatalf("match: got %q, %v; want h1", addr, err)
	}

	// A reachable host in a DIFFERENT datacenter aborts -- and must NOT skip to
	// the next host, even though that one would match. This is the dangerous
	// case (a hosts entry that points at the wrong cluster).
	addr, err := selectHost([]string{"h1", "h2"}, want, dcOf(map[string]string{"h1": "dc1", "h2": want}))
	if err == nil || addr != "" || !strings.Contains(err.Error(), "datacenter guard") {
		t.Fatalf("mismatch must abort, not skip: got %q, %v", addr, err)
	}

	// An unreachable host falls through to the next reachable one.
	if addr, err := selectHost([]string{"down", "h2"}, want, dcOf(map[string]string{"h2": want})); err != nil || addr != "h2" {
		t.Fatalf("fallthrough: got %q, %v; want h2", addr, err)
	}

	// No reachable host at all.
	if _, err := selectHost([]string{"a", "b"}, want, dcOf(map[string]string{})); err == nil || !strings.Contains(err.Error(), "reachable") {
		t.Fatalf("all unreachable: want a 'reachable' error; got %v", err)
	}
}

// pickWeighted must map a point to the choice whose cumulative-weight band it
// falls in (a boundary belongs to the later choice). A bug here skews which
// operations churn picks.
func TestPickWeighted(t *testing.T) {
	ws := []opWeight{{"a", 30}, {"b", 70}} // total 100; a covers [0,30), b covers [30,100)
	cases := []struct {
		r    float64
		want string
	}{
		{0.0, "a"},
		{0.29, "a"},
		{0.30, "b"}, // boundary belongs to b
		{0.99, "b"},
	}
	for _, c := range cases {
		if got := pickWeighted(c.r, ws); got != c.want {
			t.Errorf("pickWeighted(%.2f) = %q; want %q", c.r, got, c.want)
		}
	}
	if got := pickWeighted(0.5, []opWeight{{"only", 10}}); got != "only" {
		t.Errorf("single choice: got %q; want only", got)
	}
}
