# Surveyor

Simple tool to scrape signal statistics from a Surfboard cable modem configuration page and generate graphs. The 
goal is to complement [Smokeping][] latency graphs with signal metrics to understand network performance. Copper is 
fickle and the environment is ever-changing. It's nice to know that high latency or packet loss is caused by 
conditions in the cable itself rather than the myriad of other reasons a network isn't perfect

![Surveyor Screenshot](screenshot.png?raw=true "Surveyor Screenshot")

## Install

The easiest way to run is with [docker-compose][install-compose].

```bash
docker compose up --build
```

Assuming [go is installed][install-go], you can run it directly:

```bash
# Compile and run
go run main.go

# Compile and keep the binary to run later
go build -o bin/surveyor
```

It assumes a Surfboard model SB6141 is running its signal status webpage at `http://192.168.100.1/cmSignalData.htm` 
Although, anything that exposes a way to gather statistics could be adapted easily.

## Configuration

```bash
go run main.go --help
Usage of bin/surveyor:
  -addr string
        Listen address for the graph server (default ":8080")
  -data string
        Path to the RRD database (default "surveyor.rrd")
```

[smokeping]: https://oss.oetiker.ch/smokeping/
[install-docker]: https://docs.docker.com/get-docker/
[install-compose]: https://docs.docker.com/compose/install/
[install-go]: https://go.dev/doc/install