[![Go](https://github.com/matei-oltean/go-torrent/actions/workflows/go.yml/badge.svg)](https://github.com/matei-oltean/go-torrent/actions/workflows/go.yml)

# go-torrent

A BitTorrent client written in Go, implementing [BEP 3](https://www.bittorrent.org/beps/bep_0003.html) (core protocol) with support for:
- HTTP and UDP trackers
- Multi-file torrents
- Magnet link downloads (via DHT and trackers)
- Extension protocol (BEP 10) for metadata download
- DHT (BEP 5) for trackerless peer discovery

## Installation

```bash
git clone https://github.com/matei-oltean/go-torrent.git
cd go-torrent
go build
```

## Usage

```bash
# Download from a .torrent file
./go-torrent path/to/file.torrent

# Download from a magnet link
./go-torrent "magnet:?xt=urn:btih:..."

# Specify output directory
./go-torrent -o /path/to/output path/to/file.torrent
./go-torrent -o /path/to/output "magnet:?xt=urn:btih:..."
```

## Features

### Implemented BEPs
- [BEP 3](https://www.bittorrent.org/beps/bep_0003.html) - The BitTorrent Protocol Specification
- [BEP 5](https://www.bittorrent.org/beps/bep_0005.html) - DHT Protocol (read-only)
- [BEP 9](https://www.bittorrent.org/beps/bep_0009.html) - Extension for Peers to Send Metadata Files
- [BEP 10](https://www.bittorrent.org/beps/bep_0010.html) - Extension Protocol
- [BEP 15](https://www.bittorrent.org/beps/bep_0015.html) - UDP Tracker Protocol

### Peer Discovery
- HTTP and UDP trackers
- DHT (Distributed Hash Table) for trackerless downloads
- Peer addresses from magnet links (`x.pe` parameter)

## Roadmap

- [x] Core BitTorrent protocol (BEP 3)
- [x] HTTP/UDP tracker support
- [x] Multi-file torrents
- [x] Extension protocol (BEP 10)
- [x] DHT (BEP 5)
- [x] Magnet link downloads
- [ ] Seeding support
- [ ] GUI
