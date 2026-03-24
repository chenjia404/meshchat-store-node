FROM golang:1.26.1 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/stored ./cmd/stored

FROM debian:bookworm-slim

WORKDIR /app

RUN groupadd --system app && useradd --system --gid app --create-home --home-dir /app app

COPY --from=builder /out/stored /usr/local/bin/stored

RUN mkdir -p /app/data && chown -R app:app /app

USER app

EXPOSE 4001/tcp
EXPOSE 4001/udp

VOLUME ["/app/data"]

ENTRYPOINT ["/usr/local/bin/stored"]
