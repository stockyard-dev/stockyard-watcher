# Stockyard Watcher

**Cron job monitor — register jobs, they ping Watcher when they run, alert on missed windows**

Part of the [Stockyard](https://stockyard.dev) family of self-hosted developer tools.

## Quick Start

```bash
docker run -p 9240:9240 -v watcher_data:/data ghcr.io/stockyard-dev/stockyard-watcher
```

Or with docker-compose:

```bash
docker-compose up -d
```

Open `http://localhost:9240` in your browser.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9240` | HTTP port |
| `DATA_DIR` | `./data` | SQLite database directory |
| `WATCHER_LICENSE_KEY` | *(empty)* | Pro license key |

## Free vs Pro

| | Free | Pro |
|-|------|-----|
| Limits | 5 jobs | Unlimited jobs |
| Price | Free | $2.99/mo |

Get a Pro license at [stockyard.dev/tools/](https://stockyard.dev/tools/).

## Category

Operations & Teams

## License

Apache 2.0
