# Exatorrent Download Complete Handler

A minimal webhook service that listens for torrent completion events from exatorrent and copies the associated files from the torrent directory to a processing directory.

Built as a lightweight, zero-dependency scratch container using Go.

## How It Works

- exatorrent triggers a POST request to this service when a torrent completes.
- This service receives the torrent's infohash and:
  - Locates the completed files at /data/torrents/{infohash}
  - Recursively copies them to /data/complete/{infohash}

## Usage with Docker Compose

```yaml
version: "3.8"

services:
  exatorrent:
    image: ghcr.io/varbhat/exatorrent:latest
    ports:
      - "8088:5000"
    volumes:
      - ./exatorrent-data:/exa/exadir
    networks:
      - exatorrent_net

  copyhook:
    build:
      context: ./exatorrent-complete-handler
    volumes:
      - ./exatorrent-data/torrents:/data/torrents
      - ./complete:/data/complete
    networks:
      - exatorrent_net

networks:
  exatorrent_net:
    driver: bridge
```

Both services must share access to the appropriate directories via /data.

## exatorrent Configuration

Update your config.json in exatorrent to include:

```json 
{
    "listencompletion": true,
    "hookposturl": "http://copyhook:8080/complete",
    "notifyoncomplete": true }
```

These settings enable exatorrent to notify this service when torrents complete.

## Webhook Payload Format

The service expects the following JSON payload:

```json
{
    "metainfo": "abc123...",
    "name": "Some Torrent",
    "state": "torrent-completed-exatorrent",
    "time": "2025-04-07T00:00:00Z"
}
```

## Security

This service is designed for internal use within a Docker network. It should not be exposed to the public internet.
