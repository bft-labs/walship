# walship

![Latest Release](https://img.shields.io/github/v/release/bft-labs/cosmos-analyzer-shipper)

A lightweight agent that streams Cosmos node WAL data to [apphash.io](https://apphash.io) for consensus monitoring and debugging.

## Prerequisites

Memlogger must be integrated and enabled on your node. We ship Cosmos SDK releases with memlogger already baked in; if you run a custom fork, you can cherry-pick our single memlogger commit to enable it. For a step-by-step walkthrough, see the [Getting Started Guide](https://docs.apphash.io/getting-started), or book time via [Calendly](https://calendly.com/actor93kor/30min)—we can guide you live or handle it for you.

After integration, ensure `$NODE_HOME/config/app.toml` includes the following section:

```toml
[memlogger]
enabled = true
filter = true
interval = "2s"
```

Once enabled, WAL files will rotate under `<NODE_HOME>/data/log.wal/`.

## Installation

```bash
FILE=walship_Linux_x86_64.tar.gz  # pick the tarball for your OS/arch
curl -LO https://github.com/bft-labs/cosmos-analyzer-shipper/releases/latest/download/$FILE
curl -LO https://github.com/bft-labs/cosmos-analyzer-shipper/releases/latest/download/checksums.txt

# Verify (Linux)
grep "$FILE" checksums.txt | sha256sum --check -

# Verify (macOS)
grep "$FILE" checksums.txt | shasum -a 256 --check -

# Install
tar xzf "$FILE"
sudo mv walship /usr/local/bin/
```

Other platforms: see [Releases](https://github.com/bft-labs/cosmos-analyzer-shipper/releases).
Checksums (`checksums.txt`) are published with each release.

## Quick Start

```bash
# Get your auth key: https://apphash.io/ → create project → Project Settings.
NODE_HOME="$HOME/.osmosisd"  # e.g., ~/.neutrond, ~/.quasard
walship --node-home "$NODE_HOME" \
  --auth-key <YOUR_AUTH_KEY>
```

## Running as a Service

Create `/etc/systemd/system/walship.service`:

```ini
[Unit]
Description=Walship
After=network-online.target

[Service]
User=validator
ExecStart=/usr/local/bin/walship \
  --node-home /home/validator/.osmosisd \
  --auth-key <YOUR_AUTH_KEY>
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Adjust `User`, `--node-home`, and `--auth-key` to match your environment. If you prefer not to keep the key in the unit file, you can supply `WALSHIP_AUTH_KEY` (and other flags) via an `EnvironmentFile`.

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
| `--auth-key` | `WALSHIP_AUTH_KEY` | Project auth key from `apphash.io` → Project Settings |

### Config File

Alternatively, create `~/.walship/config.toml`:

```toml
node_home = "/home/validator/.osmosisd"
auth_key = "your-key"
```

## Additional Details

- walship auto-discovers `chain-id` and `node-id` from your node's config files and genesis.
- Data is sent to `api.apphash.io` (no custom endpoint or proxy configuration needed).
- The auth key identifies your project; keep it private even though it is not highly privileged.

## Troubleshooting

**"no index files found"**
- Ensure memlogger is enabled in `app.toml`
- Check WAL files exist in `<NODE_HOME>/data/log.wal/` (e.g., `~/.osmosisd/data/log.wal/`)

## Building from Source

Requires Go 1.22+

```bash
git clone https://github.com/bft-labs/cosmos-analyzer-shipper
cd cosmos-analyzer-shipper && make build
./walship --help
```

## Documentation

- [Getting Started](https://docs.apphash.io/getting-started) - Full setup guide
- [Node Configuration](https://docs.apphash.io/setup/configuration) - Detailed memlogger settings
- [Architecture](https://docs.apphash.io/architecture/overview) - How it works

## License

Apache-2.0. See `LICENSE`.
