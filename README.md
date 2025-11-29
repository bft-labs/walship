# walship (cosmos-analyzer-shipper)

Walship is a lightweight, operator-friendly agent that streams CometBFT WAL frames (MemLogger/WAL) to a backend system for block-level analytics, replay, debugging, and monitoring.

## Quickstart

### 1. Download binary
```bash
wget https://github.com/bft-labs/cosmos-analyzer-shipper/releases/latest/download/walship_Linux_x86_64.tar.gz
tar xzf walship_Linux_x86_64.tar.gz
chmod +x walship
```

### 2. Run walship

walship auto-discovers:
* `chain-id` → from config/genesis.json
* `node-id` → from config/node_key.json
* WAL paths → data/cs.wal/ or data/log.wal/

```bash
./walship \
  --node-home /path/to/.evmosd \
  --service-url https://api.example.com/v1/ingest \
  --auth-key=$MY_AUTH_KEY
```


## Features

- Ships compressed WAL frames (`*.wal.gz`) using `.idx` sidecar files to determine byte ranges (no full file read).
- Auto-detects node metadata (chain-id, node-id) from node files.
- Persists progress in `$HOME/.cosmos-analyzer-shipper/agent-status.json` to avoid duplicates.
- Defers sends under high CPU/network; hard interval forces progress.

## Installation

### Pre-built Binaries

Download from [Releases](https://github.com/bft-labs/cosmos-analyzer-shipper/releases):

```bash
wget https://github.com/bft-labs/cosmos-analyzer-shipper/releases/latest/download/walship_Linux_x86_64.tar.gz
tar xzf walship_Linux_x86_64.tar.gz
./walship --help
```

### Docker

```bash
docker run -d \
  --name walship \
  -v /path/to/.evmosd:/node \
  -e WALSHIP_NODE_HOME=/node \
  -e WALSHIP_SERVICE_URL=https://api.example.com/v1/ingest \
  -e WALSHIP_AUTH_KEY=$MY_AUTH_KEY \
  --restart unless-stopped \
  ghcr.io/bft-labs/cosmos-analyzer-shipper:latest
```

### Build from Source

```bash
git clone https://github.com/bft-labs/cosmos-analyzer-shipper
cd cosmos-analyzer-shipper
make build
./walship --help
```

## Configuration

Configuration is loaded in the following order (highest to lowest priority):

1. **CLI Flags** (e.g., `--service-url`)
2. **Environment Variables** (e.g., `WALSHIP_SERVICE_URL`)
3. **Config File** (default: `$HOME/.walship/config.toml`)

### Required

You must provide **either**:
- `--node-home` (node home directory) - Auto-discovers chain-id and node-id
- **OR** `--wal-dir` + `--chain-id` + `--node-id` explicitly

And:
- `--service-url`

### Environment Variables

All CLI flags have a `WALSHIP_` prefixed environment variable equivalent:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--node-home` | `WALSHIP_NODE_HOME` | (required) | Node home directory (contains `config/`, `data/`) |
| `--chain-id` | `WALSHIP_CHAIN_ID` | (auto-discovered) | Override chain ID from genesis.json |
| `--node-id` | `WALSHIP_NODE_ID` | `"default"` | Override node ID |
| `--wal-dir` | `WALSHIP_WAL_DIR` | (auto-discovered) | WAL directory path |
| `--service-url` | `WALSHIP_SERVICE_URL` | (required) | Service URL (e.g., `https://api.apphash.io/v1/ingest`) |
| `--auth-key` | `WALSHIP_AUTH_KEY` | `""` | Authorization key |
| `--poll` | `WALSHIP_POLL_INTERVAL` | `"500ms"` | Poll interval when idle |
| `--send-interval` | `WALSHIP_SEND_INTERVAL` | `"5s"` | Soft send interval |
| `--hard-interval` | `WALSHIP_HARD_INTERVAL` | `"10s"` | Hard send interval (override gating) |
| `--timeout` | `WALSHIP_HTTP_TIMEOUT` | `"15s"` | HTTP request timeout |
| `--cpu-threshold` | `WALSHIP_CPU_THRESHOLD` | `0.85` | Max CPU usage (0.0-1.0) before delaying send |
| `--net-threshold` | `WALSHIP_NET_THRESHOLD` | `0.70` | Max network usage (0.0-1.0) before delaying send |
| `--iface` | `WALSHIP_IFACE` | `""` | Network interface to monitor (optional) |
| `--iface-speed` | `WALSHIP_IFACE_SPEED` | `1000` | Interface speed in Mbps |
| `--max-batch-bytes` | `WALSHIP_MAX_BATCH_BYTES` | `4194304` | Maximum compressed bytes per batch (4MB) |
| `--state-dir` | `WALSHIP_STATE_DIR` | `$HOME/.walship` | State directory for agent-status.json |

### Config File Example

Create `$HOME/.walship/config.toml`:

```toml
node_home = "/path/to/node"
service_url = "https://api.example.com/v1/ingest"
auth_key = "your-secret-key"

poll_interval = "500ms"
send_interval = "5s"
hard_interval = "10s"

cpu_threshold = 0.85
net_threshold = 0.70
```

## Usage Examples

### Auto Discovery (most users)

Automatically discovers chain-id and node-id from node files:

```bash
./walship \
  --node-home /home/validator/.osmosisd \
  --service-url https://api.example.com/v1/ingest \
  --auth-key your-secret-key
```

Internally sends to:
- WAL frames: `POST https://api.example.com/v1/ingest/wal-frames`
- Config (future): `POST https://api.example.com/v1/ingest/config`

Node metadata (chain-id, node-id) is sent via headers:
- `X-Cosmos-Analyzer-Chain-Id`
- `X-Cosmos-Analyzer-Node-Id`


### Docker

```bash
docker run -d \
  --name walship \
  -v /path/to/.evmd:/node \
  -e WALSHIP_NODE_HOME=/node \
  -e WALSHIP_SERVICE_URL=https://api.example.com/v1/ingest \
  -e WALSHIP_AUTH_KEY=$MY_AUTH_KEY \
  --restart unless-stopped \
  ghcr.io/bft-labs/cosmos-analyzer-shipper:latest
```

## Development

```bash
make test
go test ./... -cover
make build
```

### Docker build:

```bash
# For manual use (multi-stage build from source)
docker build -t walship .
```

### Local Development

```bash
# Build
make build

# Run with debug output
./walship --node-home /path/to/node --service-url http://localhost:8080/v1/ingest --meta
```

## Troubleshooting

### "no index files found"

Ensure your WAL directory contains `.idx` files:
```bash
ls -la /path/to/data/log.wal/node-*/
```

### "chain-id is required"

Either provide `--node-home` (for auto-discovery) or explicitly set:
```bash
--chain-id my-chain --node-id abc123 --wal-dir /path/to/wal
```
