package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// --- Consul HTTP API shapes (only the fields used) ---

type agentSelf struct {
	Config struct {
		Datacenter string `json:"Datacenter"`
	} `json:"Config"`
}

type catalogNode struct {
	Node    string `json:"Node"`
	Address string `json:"Address"`
	ID      string `json:"ID"`
}

type catalogNodeServices struct {
	Services map[string]struct {
		Service string `json:"Service"`
	} `json:"Services"`
}

type registerRequest struct {
	Datacenter string           `json:"Datacenter"`
	Node       string           `json:"Node"`
	Address    string           `json:"Address"`
	ID         string           `json:"ID"`
	Service    *registerService `json:"Service,omitempty"`
}

type registerService struct {
	Service string `json:"Service"`
	ID      string `json:"ID"`
	Port    int    `json:"Port"`
}

type deregisterRequest struct {
	Datacenter string `json:"Datacenter"`
	Node       string `json:"Node"`
	ServiceID  string `json:"ServiceID,omitempty"`
}

// liveNode is a catalog node as currently registered.
type liveNode struct {
	address string
	id      string
}

// --- HTTP client ---

type client struct {
	http *http.Client
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.status, e.body) }

func (c *client) do(method, addr, path string, body []byte) ([]byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://"+addr+path, r)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, &httpError{status: resp.StatusCode, body: strings.TrimSpace(string(data))}
	}
	return data, nil
}

func (c *client) getJSON(addr, path string, out any) error {
	data, err := c.do(http.MethodGet, addr, path, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (c *client) putJSON(addr, path string, body any) error {
	data, _ := json.Marshal(body)
	_, err := c.do(http.MethodPut, addr, path, data)
	return err
}

func statusOf(err error) int {
	var he *httpError
	if errors.As(err, &he) {
		return he.status
	}
	return 0
}

// chooseHost returns the first reachable reviewed host, confirmed to be in the
// declared datacenter, by querying each host's agent for its datacenter.
func chooseHost(c *client, hosts []string, datacenter string) (string, error) {
	return selectHost(hosts, datacenter, func(addr string) (string, error) {
		var self agentSelf
		if err := c.getJSON(addr, "/v1/agent/self", &self); err != nil {
			return "", err
		}
		return self.Config.Datacenter, nil
	})
}

// selectHost is the host-selection decision, separated from the I/O so it can be
// tested directly. The first reachable host must be in the declared datacenter;
// a reachable host in a different datacenter is a misconfigured hosts list and
// aborts the run rather than being written to (it is NOT skipped to the next
// host). dcOf reports a host's live datacenter, or an error if unreachable.
func selectHost(hosts []string, datacenter string, dcOf func(string) (string, error)) (string, error) {
	var last error
	for _, addr := range hosts {
		live, err := dcOf(addr)
		if err != nil {
			last = err
			continue
		}
		if live != datacenter {
			return "", fmt.Errorf("datacenter guard: %s is in %q, inventory declares %q; refusing to write",
				addr, live, datacenter)
		}
		return addr, nil
	}
	return "", fmt.Errorf(`no host in "hosts" is reachable (last error: %v)`, last)
}

// --- live state ---

func (c *client) liveNodes(addr string) (map[string]liveNode, error) {
	var catalog []catalogNode
	if err := c.getJSON(addr, "/v1/catalog/nodes", &catalog); err != nil {
		return nil, err
	}
	out := map[string]liveNode{}
	for _, n := range catalog {
		out[n.Node] = liveNode{address: n.Address, id: n.ID}
	}
	return out, nil
}

func (c *client) liveKVKeys(addr string) (map[string]bool, error) {
	var keys []string
	if err := c.getJSON(addr, "/v1/kv/?recurse&keys", &keys); err != nil {
		if statusOf(err) == http.StatusNotFound {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = true
	}
	return out, nil
}

// servicePresence returns the set of "name@node" instances currently live.
func (c *client) servicePresence(addr string, services []Service) (map[string]bool, error) {
	onNode, err := c.serviceNamesByNode(addr, services)
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, s := range services {
		if onNode[s.Node][s.Name] {
			present[s.Name+"@"+s.Node] = true
		}
	}
	return present, nil
}

// serviceNamesByNode fetches, once per distinct node referenced by services, the
// set of service names registered on it.
func (c *client) serviceNamesByNode(addr string, services []Service) (map[string]map[string]bool, error) {
	byNode := map[string]map[string]bool{}
	for _, s := range services {
		if byNode[s.Node] != nil {
			continue
		}
		names, err := c.nodeServiceNames(addr, s.Node)
		if err != nil {
			return nil, err
		}
		byNode[s.Node] = names
	}
	return byNode, nil
}

// nodeServiceNames returns the set of service names registered on one node
// (empty if the node is absent from the catalog).
func (c *client) nodeServiceNames(addr, node string) (map[string]bool, error) {
	var doc catalogNodeServices
	err := c.getJSON(addr, "/v1/catalog/node/"+node, &doc)
	if statusOf(err) == http.StatusNotFound {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for id, svc := range doc.Services {
		name := svc.Service
		if name == "" {
			name = id
		}
		names[name] = true
	}
	return names, nil
}
