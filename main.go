// Command consul-fixture-churn generates sparse, seeded change inside a declared
// inventory and records every operation to an append-only ledger.
//
// WRITE tool, non-production clusters only. The inventory is the same
// inventory.json that consul-fixture-seed registers; this tool perturbs only
// what that file declares. See DESIGN.md / README.md.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Build information, injected at release time by GoReleaser via -ldflags
// (-X main.version / main.commit / main.date). The defaults apply to a plain
// `go build`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "", "churn.json: inventory reference and run parameters")
	timeout := flag.Duration("timeout", 5*time.Second, "per-request HTTP timeout")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("consul-fixture-churn %s (commit %s, built %s)\n", version, commit, date)
		return
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "consul-fixture-churn: --config is required")
		os.Exit(2)
	}
	if err := run(*configPath, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "consul-fixture-churn: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string, timeout time.Duration) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	invPath := filepath.Join(filepath.Dir(configPath), cfg.Inventory)
	inv, err := loadInventory(invPath)
	if err != nil {
		return err
	}
	if cfg.Seed == nil {
		return fmt.Errorf(`%s: "seed" is required`, configPath)
	}
	interval := 300.0
	if cfg.Interval != nil {
		interval = *cfg.Interval
	}
	count := 0
	if cfg.Count != nil {
		count = *cfg.Count
	}

	c := &client{http: &http.Client{Timeout: timeout}}
	addr, err := chooseHost(c, inv.Hosts, inv.Datacenter)
	if err != nil {
		return err
	}

	// Selection and jitter draw from separate streams so the operation sequence
	// stays a pure function of the seed and starting state, independent of the
	// interval (timing is not a reproduction target).
	sel := rand.New(rand.NewSource(*cfg.Seed))
	jit := rand.New(rand.NewSource(*cfg.Seed + 1))

	ch := &churner{c: c, addr: addr, inv: inv, sel: sel}
	done := 0
	for count == 0 || done < count {
		entry, err := ch.step()
		if err != nil {
			return err
		}
		if entry != nil {
			fmt.Println(ledgerLine(entry))
			done++
		}
		if count != 0 && done >= count {
			break
		}
		sleepWithJitter(interval, jit)
	}
	fmt.Fprintf(os.Stderr, "churn: %d operation(s)\n", done)
	return nil
}
