package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

var weights = []opWeight{
	{"kv-update", 50},
	{"add", 25},
	{"rereg", 15},
	{"delete", 10},
}

type opWeight struct {
	op     string
	weight int
}

// --- candidate sets (deterministically ordered) ---

func presentKVKeys(declared []string, live map[string]bool) []string {
	var out []string
	for _, k := range declared {
		if live[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func absentKVKeys(declared []string, live map[string]bool) []string {
	var out []string
	for _, k := range declared {
		if !live[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func presentServices(services []Service, present map[string]bool) []Service {
	return filterServices(services, present, true)
}

func absentServices(services []Service, present map[string]bool) []Service {
	return filterServices(services, present, false)
}

func filterServices(services []Service, present map[string]bool, want bool) []Service {
	var out []Service
	for _, s := range services {
		if present[s.Name+"@"+s.Node] == want {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name+"@"+out[i].Node < out[j].Name+"@"+out[j].Node
	})
	return out
}

func presentNodes(nodes []Node, live map[string]liveNode) []string {
	var out []string
	for _, n := range nodes {
		if _, ok := live[n.Name]; ok {
			out = append(out, n.Name)
		}
	}
	sort.Strings(out)
	return out
}

// poolItem unifies KV keys and service instances for a single weighted draw.
type poolItem struct {
	kind string // "kv" or "service"
	name string // key, or "svc@node"
}

func addPool(keys []string, svcs []Service) []poolItem {
	var pool []poolItem
	for _, k := range keys {
		pool = append(pool, poolItem{"kv", k})
	}
	for _, s := range svcs {
		pool = append(pool, poolItem{"service", s.Name + "@" + s.Node})
	}
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].kind != pool[j].kind {
			return pool[i].kind < pool[j].kind
		}
		return pool[i].name < pool[j].name
	})
	return pool
}

// --- RNG helpers ---

func (ch *churner) weightedChoice(choices []opWeight) string {
	return pickWeighted(ch.sel.Float64(), choices)
}

// pickWeighted maps r in [0,1) to a choice with probability proportional to its
// weight. Separated from the RNG so the cumulative-boundary logic can be tested
// directly: a bug here skews which operations churn picks.
func pickWeighted(r float64, choices []opWeight) string {
	total := 0
	for _, c := range choices {
		total += c.weight
	}
	point := r * float64(total)
	upto := 0.0
	for _, c := range choices {
		upto += float64(c.weight)
		if point < upto {
			return c.op
		}
	}
	return choices[len(choices)-1].op
}

func draw(r *rand.Rand, items []string) string {
	return items[r.Intn(len(items))]
}

func randomID(r *rand.Rand) string {
	s := fmt.Sprintf("%016x%016x", r.Uint64(), r.Uint64())
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}

func sleepWithJitter(interval float64, r *rand.Rand) {
	if interval <= 0 {
		return
	}
	factor := 1.0 + (r.Float64() - 0.5)
	time.Sleep(time.Duration(interval * factor * float64(time.Second)))
}
