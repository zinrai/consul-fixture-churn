package main

import (
	"fmt"
	"math/rand"
	"net/http"
)

type ledgerEntry struct {
	verb, kind, name, detail string
}

type churner struct {
	c    *client
	addr string
	inv  *Inventory
	sel  *rand.Rand
}

// step performs one operation and returns its ledger entry, or nil if no
// operation has an eligible target this tick.
func (ch *churner) step() (*ledgerEntry, error) {
	live, err := ch.c.liveNodes(ch.addr)
	if err != nil {
		return nil, err
	}
	liveKV, err := ch.c.liveKVKeys(ch.addr)
	if err != nil {
		return nil, err
	}
	present, err := ch.c.servicePresence(ch.addr, ch.inv.Services)
	if err != nil {
		return nil, err
	}

	upKeys := presentKVKeys(ch.inv.KV, liveKV)
	addKeys := absentKVKeys(ch.inv.KV, liveKV)
	addSvcs := absentServices(ch.inv.Services, present)
	delSvcs := presentServices(ch.inv.Services, present)
	reregNodes := presentNodes(ch.inv.Nodes, live)

	eligible := map[string]bool{
		"kv-update": len(upKeys) > 0,
		"add":       len(addKeys) > 0 || len(addSvcs) > 0,
		"delete":    len(upKeys) > 0 || len(delSvcs) > 0,
		"rereg":     len(reregNodes) > 0,
	}
	var available []opWeight
	for _, ow := range weights {
		if eligible[ow.op] {
			available = append(available, ow)
		}
	}
	if len(available) == 0 {
		return nil, nil
	}

	switch ch.weightedChoice(available) {
	case "kv-update":
		return ch.doKVUpdate(draw(ch.sel, upKeys))
	case "rereg":
		name := draw(ch.sel, reregNodes)
		return ch.doRereg(name, live[name])
	case "add":
		return ch.doAdd(addKeys, addSvcs, live)
	default:
		return ch.doDelete(upKeys, delSvcs)
	}
}

func (ch *churner) doKVUpdate(key string) (*ledgerEntry, error) {
	raw, err := ch.c.do(http.MethodGet, ch.addr, "/v1/kv/"+key+"?raw", nil)
	if err != nil {
		return nil, err
	}
	rev := parseRev(string(raw)) + 1
	if _, err := ch.c.do(http.MethodPut, ch.addr, "/v1/kv/"+key, kvValue(key, rev)); err != nil {
		return nil, err
	}
	return &ledgerEntry{"update", "kv", key, fmt.Sprintf("rev=%d", rev)}, nil
}

func (ch *churner) doAdd(addKeys []string, addSvcs []Service, live map[string]liveNode) (*ledgerEntry, error) {
	pool := addPool(addKeys, addSvcs)
	pick := pool[ch.sel.Intn(len(pool))]
	if pick.kind == "kv" {
		if _, err := ch.c.do(http.MethodPut, ch.addr, "/v1/kv/"+pick.name, kvValue(pick.name, 1)); err != nil {
			return nil, err
		}
		return &ledgerEntry{"add", "kv", pick.name, "rev=1"}, nil
	}
	svc, node := splitInstance(pick.name)
	ln := live[node]
	body := registerRequest{
		Datacenter: ch.inv.Datacenter, Node: node, Address: ln.address, ID: ln.id,
		Service: &registerService{Service: svc, ID: svc, Port: 80},
	}
	if err := ch.c.putJSON(ch.addr, "/v1/catalog/register", body); err != nil {
		return nil, err
	}
	return &ledgerEntry{"add", "service", pick.name, ""}, nil
}

func (ch *churner) doDelete(delKeys []string, delSvcs []Service) (*ledgerEntry, error) {
	pool := addPool(delKeys, delSvcs)
	pick := pool[ch.sel.Intn(len(pool))]
	if pick.kind == "kv" {
		if _, err := ch.c.do(http.MethodDelete, ch.addr, "/v1/kv/"+pick.name, nil); err != nil {
			return nil, err
		}
		return &ledgerEntry{"delete", "kv", pick.name, ""}, nil
	}
	svc, node := splitInstance(pick.name)
	body := deregisterRequest{Datacenter: ch.inv.Datacenter, Node: node, ServiceID: svc}
	if err := ch.c.putJSON(ch.addr, "/v1/catalog/deregister", body); err != nil {
		return nil, err
	}
	return &ledgerEntry{"delete", "service", pick.name, ""}, nil
}

func (ch *churner) doRereg(name string, ln liveNode) (*ledgerEntry, error) {
	newID := randomID(ch.sel)
	body := registerRequest{Datacenter: ch.inv.Datacenter, Node: name, Address: ln.address, ID: newID}
	if err := ch.c.putJSON(ch.addr, "/v1/catalog/register", body); err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("id=old:%s new:%s", short(ln.id), short(newID))
	return &ledgerEntry{"rereg", "node", name, detail}, nil
}
