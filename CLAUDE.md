# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Surveyor is a Prometheus exporter for the signal statistics of an Arris SURFboard DOCSIS 3.1 cable modem (downstream SNR, power, codeword errors; upstream power). It runs inside the panorama stack (`../panorama`), which builds it from this checkout. See the README for configuration and the metric list.

## Architecture

- `main.go`: configuration (flags plus `SURVEYOR_MODEM_*` env vars), signal handling, HTTP server
- `surveyor/hnap.go`: HNAP client. HMAC-MD5 login, one bundled `GetMultipleHNAPs` request for downstream, upstream, and software info
- `surveyor/channelinfo.go`: parsers for the `^`-delimited channel records
- `surveyor/poller.go`: background loop that owns all modem traffic, with exponential backoff
- `surveyor/collector.go`: Prometheus collector that only reads the poller's snapshot

The modem is never contacted from inside a scrape. That used to be the design, and it meant every reader of `/metrics` added modem load and a slow modem failed the scrape.

## Modem Behavior

Observed on the real modem, and reproduced by the fake in `surveyor/client_test.go`:

- A wrong password gets **HTTP 200** with `"LoginResult": "FAILED"`. The body has to be checked.
- Any request with a missing or expired session gets **HTTP 404**.
- The downstream query is slow and scales with bonded channels: about 2.5s at 16 channels, 5.5s at 32. Upstream and software info add well under a second.
- The modem keeps working on requests the client abandoned, and drops connections when two clients talk to it at once. Overlapping or rapid retries wedge its web server until a reboot.
- `HNAP_AUTH` carries a millisecond timestamp, matching the web UI's `Date.now() % 2000000000000`.

## Development Commands

```bash
SURVEYOR_MODEM_PASSWORD=... go run . -addr 127.0.0.1:18080   # against the real modem
go test ./...                                                # includes a fake TLS modem
go test -race ./...
go vet ./...
STATICCHECK_CACHE=/tmp/staticcheck staticcheck ./...         # default cache dir is not writable under the sandbox
```

Running locally while the stack is up means two pollers hit the modem. That is fine for a quick check, but use a long `-interval` and stop it when done.

## Deploying

From `../panorama`: `scripts/stack build surveyor && scripts/stack up surveyor`, then `scripts/stack health`.
