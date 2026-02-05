[![Go](https://github.com/matei-oltean/go-torrent/actions/workflows/go.yml/badge.svg)](https://github.com/matei-oltean/go-torrent/actions/workflows/go.yml)

# go-torrent

A BitTorrent client written in Go, implementing [BEP 3](https://www.bittorrent.org/beps/bep_0003.html) (core protocol) with support for:
- HTTP and UDP trackers
- Multi-file torrents
- Magnet link parsing
- Extension protocol (BEP 10) for metadata download

## Installation

```bash
git clone https://github.com/matei-oltean/go-torrent.git
cd go-torrent
go build
```

## Usage

```bash
# Download from a .torrent file
./go-torrent -f path/to/file.torrent

# Specify output directory
./go-torrent -f path/to/file.torrent -o /path/to/output
```

## Roadmap

- [x] Core BitTorrent protocol (BEP 3)
- [x] HTTP/UDP tracker support
- [x] Multi-file torrents
- [x] Extension protocol (BEP 10)
- [ ] DHT (BEP 5) - *in progress*
- [ ] Magnet link downloads
- [ ] Seeding support
- [ ] GUI
