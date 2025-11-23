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
  --root /path/to/.evmosd \
  --remote-url https://api.example.com/v1/ingest \
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
  -e WALSHIP_ROOT=/node \
  -e WALSHIP_REMOTE_URL=https://api.example.com/v1/ingest \
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

1. **CLI Flags** (e.g., `--remote-url`)
2. **Environment Variables** (e.g., `WALSHIP_REMOTE_URL`)
3. **Config File** (default: `$HOME/.walship/config.toml`)

### Required

You must provide **either**:
- `--root` (node root directory) - Auto-discovers chain-id and node-id
- **OR** `--wal-dir` + `--chain-id` + `--node-id` explicitly

And:
- `--remote-url` or (`--remote-base` + `--network`)

### Environment Variables

All CLI flags have a `WALSHIP_` prefixed environment variable equivalent:

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--root` | `WALSHIP_ROOT` | Node root directory (contains `config/`, `data/`) |
| `--chain-id` | `WALSHIP_CHAIN_ID` | Override chain ID from genesis.json |
| `--node-id` | `WALSHIP_NODE` | Override node ID (defaults to "default") |
| `--wal-dir` | `WALSHIP_WAL_DIR` | WAL directory path |
| `--remote-url` | `WALSHIP_REMOTE_URL` | Full remote endpoint URL |
| `--remote-base` | `WALSHIP_REMOTE_BASE` | Remote base URL |
| `--network` | `WALSHIP_NETWORK` | Network identifier |
| `--auth-key` | `WALSHIP_AUTH_KEY` | Authorization key |
| `--poll` | `WALSHIP_POLL_INTERVAL` | Poll interval (e.g., "500ms") |
| `--send-interval` | `WALSHIP_SEND_INTERVAL` | Soft send interval |
| `--cpu-threshold` | `WALSHIP_CPU_THRESHOLD` | Max CPU usage (0.0-1.0) |
| `--net-threshold` | `WALSHIP_NET_THRESHOLD` | Max network usage (0.0-1.0) |

### Config File Example

Create `$HOME/.walship/config.toml`:

```toml
root = "/path/to/node"
remote_url = "http://backend:8080/v1/ingest/mychain/validator-01/wal-frames"
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
  --root /home/validator/.osmosisd \
  --remote-base https://api.example.com \
  --network mainnet \
  --auth-key your-secret-key
```
URL becomes:
`https://api.example.com/v1/ingest/mainnet/{node-id}/wal-frames`


### Docker

```bash
docker run -d \
  --name walship \
  -v /path/to/.evmd:/node \
  -e WALSHIP_ROOT=/node \
  -e WALSHIP_REMOTE_URL=https://api.example.com/v1/ingest \
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
./walship --root /path/to/node --remote-url http://localhost:8080 --meta
```

## Troubleshooting

### "no index files found"

Ensure your WAL directory contains `.idx` files:
```bash
ls -la /path/to/data/log.wal/node-*/
```

### "chain-id is required"

Either provide `--root` (for auto-discovery) or explicitly set:
```bash
--chain-id my-chain --node-id abc123 --wal-dir /path/to/wal
```
