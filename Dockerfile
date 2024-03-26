FROM golang:1.22.1-bookworm as builder
WORKDIR /app
ADD . /app

# Append a suffix to prevent colliding with the directory of the same name
RUN go build -o surveyor-build

FROM debian:bookworm-slim
COPY --from=builder /app/surveyor-build /app/surveyor

WORKDIR /app
CMD ["/app/surveyor"]
