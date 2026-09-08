FROM golang:1.27-trixie AS builder
WORKDIR /app
ADD . /app

# Append a suffix to prevent colliding with the directory of the same name
RUN go build -o surveyor-build

FROM debian:trixie-slim
COPY --from=builder /app/surveyor-build /app/surveyor

WORKDIR /app
ENTRYPOINT ["/app/surveyor"]
