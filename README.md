# walship

![Latest Release](https://img.shields.io/github/v/release/bft-labs/walship)

A lightweight agent that streams Cosmos node WAL data to [apphash.io](https://apphash.io) for consensus monitoring and debugging.

## Prerequisites

> **Note for Chain Developers Only**
>
> This section is for chain developers who need to integrate apphash.io functionality into their chain binary. If you're an operator or partner running an already-integrated chain binary, skip this section and proceed directly to [Installation](#installation) and [Running as a Service](#running-as-a-service-recommended).

Memlogger must be integrated and enabled on your node. We ship Cosmos SDK releases with memlogger already baked in; if you run a custom fork, you can cherry-pick our single memlogger commit to enable it. For a step-by-step walkthrough, see the [Getting Started Guide](https://docs.apphash.io/getting-started), or book time via [Calendly](https://calendly.com/actor93kor/30min)—we can guide you live or handle it for you.

After integration, ensure `$NODE_HOME/config/app.toml` includes the following section:

```toml
[memlogger]
enabled = true
filter = true
interval = "2s"
```

Once enabled, WAL files will rotate under `<NODE_HOME>/data/log.wal/`.

---

## Installation

```bash
FILE=walship_Linux_x86_64.tar.gz  # pick the tarball for your OS/arch
curl -LO https://github.com/bft-labs/walship/releases/latest/download/$FILE
curl -LO https://github.com/bft-labs/walship/releases/latest/download/checksums.txt

# Verify (Linux)
grep "$FILE" checksums.txt | sha256sum --check -

# Verify (macOS)
grep "$FILE" checksums.txt | shasum -a 256 --check -

# Install
tar xzf "$FILE"
sudo mv walship /usr/local/bin/
```

Other platforms: see [Releases](https://github.com/bft-labs/walship/releases).
Checksums (`checksums.txt`) are published with each release.

## Quick Start (Not Recommended)

> **⚠️ Not recommended for production use.** Use [Running as a Service](#running-as-a-service-recommended) instead for better reliability and automatic restarts.

```bash
# Get your auth key: https://apphash.io/ → create project → Project Settings.
NODE_HOME="$HOME/.osmosisd"  # e.g., ~/.neutrond, ~/.quasard
walship --node-home "$NODE_HOME" \
  --chain-binary-path /path/to/osmosisd \
  --chain-id osmosis-1 \
  --auth-key <YOUR_AUTH_KEY>
```

> **Tip:** If you prefer not to use `--chain-binary-path`, you can pass `--node-id <hex>` directly instead.

## Running as a Service (RECOMMENDED)

Create `/etc/systemd/system/walship.service`:

```ini
[Unit]
Description=Walship
After=network-online.target

[Service]
User=validator
ExecStart=/usr/local/bin/walship \
  --node-home /home/validator/.osmosisd \
  --chain-binary-path /usr/local/bin/osmosisd \
  --chain-id osmosis-1 \
  --auth-key <YOUR_AUTH_KEY>
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Adjust `User`, `--node-home`, `--chain-binary-path`, `--chain-id`, and `--auth-key` to match your environment. If you prefer not to keep the key in the unit file, you can supply `WALSHIP_AUTH_KEY` (and other flags) via an `EnvironmentFile`.

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now walship
sudo journalctl -u walship -f  # view logs
```


## Configuration

Essential flags are below; run `walship -h` to see the full list. All flags can be set via environment variables with `WALSHIP_` prefix.

### Required

| Flag | Env | Description |
|------|-----|-------------|
| `--node-home` | `WALSHIP_NODE_HOME` | Node home directory (e.g., `~/.osmosisd`, `~/.<binary>d`) |
| `--chain-id` | `WALSHIP_CHAIN_ID` | Chain ID (e.g., `osmosis-1`, `evmos_9001-2`) |
| `--auth-key` | `WALSHIP_AUTH_KEY` | Project auth key from `apphash.io` → Project Settings |
| `--chain-binary-path` | `WALSHIP_CHAIN_BINARY_PATH` | Path to chain binary (e.g., `osmosisd`). Derives node ID via `comet show-node-id` |

### Config File

Alternatively, create `~/.walship/config.toml`:

```toml
node_home = "/home/validator/.osmosisd"
chain_binary_path = "/usr/local/bin/osmosisd"
chain_id = "osmosis-1"
auth_key = "your-key"
```

## Additional Details

- walship never reads any key files (`node_key.json`, `priv_validator_key.json`). Node ID is derived by executing the chain binary (`comet show-node-id`) or supplied directly via `--node-id`.
- Data is sent to `api.apphash.io` (no custom endpoint or proxy configuration needed).
- The auth key identifies your project; keep it private even though it is not highly privileged.

## Troubleshooting

**"no index files found"**
- Ensure memlogger is enabled in `app.toml`
- Check WAL files exist in `<NODE_HOME>/data/log.wal/` (e.g., `~/.osmosisd/data/log.wal/`)

## Building from Source

Requires Go 1.22+

```bash
git clone https://github.com/bft-labs/walship
cd walship && make build
./walship --help
```

## Documentation

- [Getting Started](https://docs.apphash.io/getting-started) - Full setup guide
- [Node Configuration](https://docs.apphash.io/setup/configuration) - Detailed memlogger settings
- [Architecture](https://docs.apphash.io/architecture/overview) - How it works

## License

Apache-2.0. See `LICENSE`.
