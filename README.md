# tonnel-bid-logger

Lightweight Go bot that scans `marketplace.tonnel.network` for bid opportunities and posts matches to a Telegram chat via a bot.

> Short: place `config.json` next to the binary and run. Be considerate of rate limits and the marketplace's terms of service.

## Features

- Fetches gifts / listings from the Tonnel marketplace.
- Filters by configurable minimum profit (TON and percent).
- Filters by rare backgrounds.
- Concurrent fetching with worker pool.
- Optional HTTP proxy list.
- Logs matches to a Telegram bot chat.
- **Respected rate limits**: exponential backoff + jitter for errors and 429 responses.
- **Proxy pool**: rotates proxies round-robin, but avoid short-lived rotations that look abusive.
- **Retries**: implemented a retry policy with a small max attempts (e.g., 3) and increasing delay

---

## Required `config.json`

Place a `config.json` next to the binary.
```json
{
    "gifts_per_fetch": 30,
    "concurrent_requests": 5,
    "min_profit": 0,
    "min_profit_ton": 1.0,
    "rare_backgrounds": ["Black", "Onyx Black"],
    "proxies": [],
    "token": "<TELEGRAM_BOT_TOKEN>",
    "chat_id": 123456789
}
```

Fields:
- `gifts_per_fetch` — how many listings to request per marketplace fetch.
- `concurrent_requests` — number of concurrent HTTP workers.
- `min_profit` — minimal profit in percent (0 = disabled).
- `min_profit_ton` — minimal profit in TON (float).
- `rare_backgrounds` — background names to treat as "rare".
- `proxies` — array of proxy URLs (examples below).
- `token` — Telegram bot token.
- `chat_id` — Telegram chat ID (numeric).

### Example proxies formats
```json
[
    "http://user:pass@1.2.3.4:3128",
    "socks5://5.6.7.8:1080",
    "http://8.8.8.8:8080"
]
```
---

## Quick start (build & run)
```sh
# build
go build -o tonnel-bid-logger .

# run (default looks for ./config.json)
./tonnel-bid-logger
```
---

## Docker
```sh
# build
docker build -t tonnellog:local .
# run
docker run -d \
  --name tonnel-logger \
  --restart unless-stopped \
  -v "$(pwd)/config.json":/root/config.json:ro \
  tonnellog:local
```
---

## License

Apache 2.0 — use and adapt at your own risk.

---
