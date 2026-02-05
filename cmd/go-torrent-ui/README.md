# Go Torrent UI

A desktop GUI for the Go Torrent client, built with [Wails](https://wails.io/) (Go + React).

## Features

- Add torrents via magnet links
- Real-time download progress with animated progress bars
- Download/upload speed display
- Peer count monitoring
- Light/dark theme toggle
- Clean, modern UI with Tailwind CSS

## Requirements

- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Development

```bash
cd cmd/go-torrent-ui
wails dev
```

This starts a development server with hot reload for frontend changes.

## Building

```bash
wails build
```

This creates a production executable in `build/bin/`.

## Stack

- **Backend**: Go + Wails
- **Frontend**: React + TypeScript
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
- **Build**: Vite
