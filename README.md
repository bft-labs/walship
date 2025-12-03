# walship

A lightweight agent that streams Cosmos node WAL data to [apphash.io](https://apphash.io) for consensus monitoring and debugging.

## Prerequisites

Enable memlogger in your node's `app.toml`:

```toml
[memlogger]
enabled = true
filter = true
interval = "2s"
```

> For full node setup (Cosmos SDK integration, app.go changes), see the [Getting Started Guide](https://apphash-docs.vercel.app/getting-started).

## Installation

```bash
# Download
curl -LO https://github.com/bft-labs/walship/releases/latest/download/walship_Linux_x86_64.tar.gz
tar xzf walship_Linux_x86_64.tar.gz

# Install
sudo mv walship /usr/local/bin/
```

Other platforms: see [Releases](https://github.com/bft-labs/walship/releases).

## Quick Start

```bash
walship \
  --node-home ~/.osmosisd \
  --service-url https://api.apphash.io \
  --auth-key <YOUR_AUTH_KEY>
```

walship auto-discovers `chain-id` and `node-id` from your node's config files.

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
  --service-url https://api.apphash.io \
  --auth-key <YOUR_AUTH_KEY>
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now walship
sudo journalctl -u walship -f  # view logs
```

## Configuration

All flags can be set via environment variables with `WALSHIP_` prefix.

### Required

| Flag | Env | Description |
|------|-----|-------------|
| `--node-home` | `WALSHIP_NODE_HOME` | Node home directory (e.g., `~/.osmosisd`) |
| `--service-url` | `WALSHIP_SERVICE_URL` | `https://api.apphash.io` |
| `--auth-key` | `WALSHIP_AUTH_KEY` | Your project auth key |

### Optional

| Flag | Default | Description |
|------|---------|-------------|
| `--poll` | `500ms` | Poll interval for new WAL files |
| `--send-interval` | `5s` | Batch send interval |
| `--cpu-threshold` | `0.85` | Pause sending above this CPU usage |
| `--net-threshold` | `0.70` | Pause sending above this network usage |

### Config File

Alternatively, create `~/.walship/config.toml`:

```toml
node_home = "/home/validator/.osmosisd"
service_url = "https://api.apphash.io"
auth_key = "your-key"
```

## Verify It's Working

```bash
# Check walship logs
sudo journalctl -u walship -f

# You should see:
# config watcher: sent configuration update
# Successfully sent batch of N frames (XXX bytes)
```

## Troubleshooting

**"no index files found"**
- Ensure memlogger is enabled in `app.toml`
- Check WAL files exist: `ls ~/.osmosisd/data/log.wal/`

**"connection refused"**
- Verify `--service-url` is correct
- Check network connectivity to api.apphash.io

## Building from Source

```bash
git clone https://github.com/bft-labs/walship
cd walship && make build
./walship --help
```

## Documentation

- [Getting Started](https://apphash-docs.vercel.app/getting-started) - Full setup guide
- [Node Configuration](https://apphash-docs.vercel.app/setup/configuration) - Detailed memlogger settings
- [Architecture](https://apphash-docs.vercel.app/architecture/overview) - How it works

## License

TBD
