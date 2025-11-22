# walship (cosmos-analyzer-shipper)

## Overview
- Ships MemLogger/WAL frames using `.idx` sidecar metadata.
- Reads compressed frames from `.wal.gz` by byte range and posts them in batches.
- Persists progress in `$HOME/.cosmos-analyzer-shipper/agent-status.json` to avoid duplicates.
- Defers sends under high CPU/network; hard interval forces progress.

## Installation

### Pre-built Binaries
Download the latest release from the [Releases page](https://github.com/bft-labs/cosmos-analyzer-shipper/releases).

### Docker
```shell
docker pull ghcr.io/bft-labs/cosmos-analyzer-shipper:latest
```

## Configuration
Configuration can be provided via:
1. **Flags**: Highest priority (e.g., `--remote-url`)
2. **Environment Variables**: `WALSHIP_` prefix (e.g., `WALSHIP_REMOTE_URL`)
3. **Config File**: Default `$HOME/.walship/config.toml` or via `--config`

### Environment Variables
All flags have a corresponding environment variable with the `WALSHIP_` prefix.
- `WALSHIP_REMOTE_URL`
- `WALSHIP_AUTH_KEY`
- `WALSHIP_WAL_DIR`
- `WALSHIP_NODE`
- ...and so on.

## Run examples

### Docker
```shell
docker run -d \
  -v /var/log/cometbft/wal:/wal \
  -e WALSHIP_WAL_DIR=/wal \
  -e WALSHIP_REMOTE_URL=http://backend:8080/... \
  -e WALSHIP_AUTH_KEY=secret \
  ghcr.io/bft-labs/cosmos-analyzer-shipper:latest
```

### Explicit endpoint
```shell
./walship \
  --wal-dir /var/log/cometbft/wal \
  --remote-url http://backend:8080/v1/ingest/<network>/<node>/wal-frames \
  --auth-key <key>
```

### Build route from base + network/node
```shell
./walship \
  --wal-dir /var/log/cometbft/wal \
  --remote-base http://backend:8080 \
  --network mychain \
  --node validator-01 \
  --auth-key <key>
```
