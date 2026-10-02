# Environment: disposable Linode

Provisioned for Checkpoint 0 experiments. This machine is intentionally
disposable and is the sanctioned place for invasive experiments, service
configuration, and dogfooding. It is **not** the maintainer's workstation.

## Linode

| Item | Value |
| --- | --- |
| Label | `switchyard-cp0` |
| ID | 107259017 |
| Region | `us-east` (Newark, NJ) |
| Plan | `g6-nanode-1` (Nanode 1GB: 1 vCPU, 1 GB RAM, 25 GB disk) |
| Monthly cost | USD ~5 |
| Image | `linode/ubuntu24.04` (Ubuntu 24.04 LTS) |
| IPv4 | `45.79.189.46` |
| Created | 2026-10-02T09:24:28Z |
| Status | running |
| Root SSH | ed25519 key from the operator's workstation (`~/.ssh/id_ed25519`) |

No Cloud Firewall is attached to the account. `ufw` is not enabled; the host
currently relies on SSH key auth only. This is acceptable for a short-lived
experimental box and was deliberately not turned into a hardening project.

## Local SSH configuration

Recorded on the operator machine in `~/.ssh/config`:

```
Host switchyard-cp0
    HostName 45.79.189.46
    User root
    IdentityFile /home/nick/.ssh/id_ed25519
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

`ssh switchyard-cp0` reaches the box.

## Toolchain installed

| Tool | Version |
| --- | --- |
| Ubuntu | 24.04 LTS, kernel 6.8.0-134 |
| Go | 1.27.1 (`/usr/local/go/bin/go`) |
| Node.js | 22.23.3 |
| npm | 10.9.9 |
| wrangler | 4.146.0 (global) |
| gcc/g++ | 13.3.0 |
| cmake | 3.28.3 |
| ninja | 1.11.1 |
| git | 2.43.0 |
| sqlite3 | 3.45.1 |
| Swap | 2 GiB `/swapfile` (added; the 1 GB box needs it for compiler builds) |

Reproducible install recipe: Ubuntu base + `build-essential cmake ninja-build
pkg-config git curl jq unzip ca-certificates libcurl4-openssl-dev libssl-dev
libsqlite3-dev python3` + NodeSource Node 22 + Go tarball + 2G swapfile.

## Workspace layout on the Linode

| Path | Content |
| --- | --- |
| `/opt/cp0/trestle` | Trestle source clone (tag `v0.1.5` = `3c58fd58` checked out; also `cmd/dialprobe` diagnostic) |
| `/opt/cp0/trestle-v015` | Trestle binary built from `v0.1.5` |
| `/opt/cp0/strut` | Strut source clone at `c4356bea` (current main) |
| `/opt/cp0/strut/build/strut` | Strut compiler built from that commit (Debug) |
| `/opt/cp0/strut-probes/` | Switchyard-surface probe programs + runner |
| `/opt/cp0/repro/` | Reproduction scripts (`lib.sh`, `standalone.sh`, `cluster_setup_v3.sh`, `cluster_test.sh`, webhook receiver) |
| `/opt/cp0/evidence/` | Captured evidence (standalone vs cluster) |
| `/opt/cp0/cluster/{a,b,c}` | Three-node local Trestle cluster (HTTP 7333-7335, Raft 7440-7442) |
| `/opt/cp0/standalone` | Standalone Trestle baseline instance |

## Cost note

Account balance was ~USD 5.51 at provisioning time. The Nanode bills ~USD 5/mo
and will accrue from 2026-10-02. Terminate with
`linode-cli linodes delete 107259017` when experiments are done.

## How to reproduce this environment

1. `linode-cli linodes create --label switchyard-cp0 --region us-east --type g6-nanode-1 --image linode/ubuntu24.04 --authorized_keys "$(cat ~/.ssh/id_ed25519.pub)"`
2. Add the `~/.ssh/config` alias above; wait for status `running`; `ssh-keyscan` the host.
3. Run the toolchain install steps from the table above.
4. Re-run the reproduction scripts under `trestle-clustering-repro/` and
   `strut-surface-validation/` (they are written to run from `/opt/cp0`).