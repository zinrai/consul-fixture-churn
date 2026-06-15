package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Inventory is the declared set of objects, the same shape consul-fixture-seed
// reads. consul-fixture-seed owns this format; see its DESIGN.md.
type Inventory struct {
	Datacenter string    `json:"datacenter"`
	Hosts      []string  `json:"hosts"`
	Nodes      []Node    `json:"nodes"`
	Services   []Service `json:"services"`
	KV         []string  `json:"kv"`
}

type Node struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Service struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

func loadInventory(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if inv.Datacenter == "" {
		return nil, fmt.Errorf(`%s: "datacenter" must be a non-empty string`, path)
	}
	if len(inv.Hosts) == 0 {
		return nil, fmt.Errorf(`%s: "hosts" must list at least one Consul HTTP address`, path)
	}
	declared := map[string]bool{}
	for _, n := range inv.Nodes {
		declared[n.Name] = true
	}
	for _, s := range inv.Services {
		if !declared[s.Node] {
			return nil, fmt.Errorf("%s: service %s placed on undeclared node %s", path, s.Name, s.Node)
		}
	}
	return &inv, nil
}
