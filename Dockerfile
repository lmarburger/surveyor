FROM golang:1.22.1-bookworm as builder
WORKDIR /app
ADD . /app

# Append a suffix to prevent colliding with the directory of the same name
RUN go build -o surveyor-build

FROM debian:bookworm-slim
COPY --from=builder /app/surveyor-build /app/surveyor
VOLUME /app/data
VOLUME /app/static

RUN apt-get update && apt-get install -y rrdtool

WORKDIR /app
CMD ["/app/surveyor", "--data", "data/surveyor.rrd"]
