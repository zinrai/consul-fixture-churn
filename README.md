# consul-fixture-churn

> **WRITE TOOL: non-production clusters only.** This changes service instances,
> KV keys, and catalog nodes on a Consul cluster. Point it at a staging or lab
> cluster, never at production. The write destination is the reviewed `hosts`
> list in `inventory.json`, not a command-line argument, so a mistyped host
> cannot reach production.

Generate sparse, seeded change inside a declared inventory, recording every
operation it performs. The inventory is the same `inventory.json`
that [consul-fixture-seed](https://github.com/zinrai/consul-fixture-seed)
registers; this tool perturbs only what that file declares, and never invents a
name.

Use it to put reproducible background change on a Consul cluster whose state was
established with consul-fixture-seed.

## churn.json

The whole format is the `ChurnConfig` struct at the top of `main.go`: a reference
to the inventory file (resolved relative to `churn.json`) plus run parameters.

```json
{
  "inventory": "inventory.json",
  "seed": 42,
  "interval": 300,
  "count": 0
}
```

- **inventory**: path to the `inventory.json` to perturb. Its format is defined
  by [consul-fixture-seed](https://github.com/zinrai/consul-fixture-seed); see
  there.
- **seed** (required): the RNG seed. The same seed against the same starting
  state reproduces the same run.
- **interval**: seconds between operations (default 300).
- **count**: stop after N operations (default 0 = run until interrupted).

`churn.json` is the only source of these parameters; there are no command-line
overrides.

## Usage

```
consul-fixture-churn --config examples/churn.json
```

Each operation prints to stdout, one line per operation; progress and the final
count go to stderr. Redirect stdout to keep a ledger:

```
consul-fixture-churn --config examples/churn.json > churn.log
```

## Write controls

- **datacenter guard**: the chosen host's `/v1/agent/self` must report the
  inventory's `datacenter` (give the verification cluster a distinct name like
  `dc2-verify`).

## License

This project is licensed under the [MIT License](./LICENSE).
