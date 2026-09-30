# Surveyor

Prometheus exporter for the signal statistics of a SURFboard cable modem. The goal is to complement
[Smokeping][]-style latency graphs with signal metrics, to tell whether high latency or packet loss is
caused by conditions in the cable itself rather than the myriad of other reasons a network isn't perfect.

![Surveyor Screenshot](screenshot.png?raw=true "Surveyor Screenshot")

Surveyor logs in to the modem's HNAP API (the same one its web UI uses), polls downstream and upstream
channel status in the background, and serves the latest result at `/metrics`. It is developed against an
Arris SURFboard DOCSIS 3.1 modem at `https://192.168.100.1/HNAP1/`.

In practice it runs as part of the [panorama][] monitoring stack, which builds it from a sibling checkout.

## Running

```bash
SURVEYOR_MODEM_PASSWORD=... go run .
curl localhost:8080/metrics
```

## Configuration

| Setting | Default | Purpose |
|---------|---------|---------|
| `SURVEYOR_MODEM_PASSWORD` | none, required | Modem admin password |
| `SURVEYOR_MODEM_USERNAME` / `-modem-username` | `admin` | Modem admin username |
| `SURVEYOR_MODEM_URL` / `-modem-url` | `https://192.168.100.1/HNAP1/` | Modem HNAP endpoint |
| `-addr` | `:8080` | Listen address for `/metrics` |
| `-interval` | `30s` | Time between modem polls |
| `-timeout` | `15s` | Deadline for one poll, including login |
| `-max-backoff` | `5m` | Longest wait between polls while the modem is failing |

Polls back off exponentially while the modem is failing. The modem keeps working on requests the client
has given up on, so retrying at the normal rate only deepens its backlog.

## Metrics

Per channel, labelled by `channel_id`, `frequency_mhz`, and `modulation` (downstream) or `type` (upstream):

- `surveyor_downstream_{frequency_hertz,power_dbmv,snr_db,locked}`
- `surveyor_downstream_{corrected,uncorrectable}_codewords_total`
- `surveyor_upstream_{frequency_hertz,power_dbmv,symbol_rate,locked}`

The ISP can renumber channels, so `frequency_mhz` is the stable way to follow one channel over time.
Channel metrics are withheld once the last successful poll is older than three intervals, so an
unreachable modem shows up as a gap rather than a flat line.

Exporter health:

- `surveyor_modem_up`: 1 if the most recent poll succeeded
- `surveyor_last_success_timestamp_seconds`, `surveyor_poll_consecutive_failures`
- `surveyor_poll_errors_total{stage="login|fetch|parse"}`
- `surveyor_poll_duration_seconds` (histogram)
- `surveyor_skipped_records_total{direction}`: channel records that could not be parsed
- `surveyor_modem_info{docsis,hardware,firmware}`: always 1; watch it for ISP firmware pushes

[smokeping]: https://oss.oetiker.ch/smokeping/
[panorama]: https://github.com/lmarburger/panorama
