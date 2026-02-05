package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/matei-oltean/go-torrent/torrent"
)

func usage() {
	fmt.Printf(`%s [options] <torrent-file|magnet-link>

    torrent-file       Path of the torrent file
    magnet-link        Magnet link (starting with magnet:)

    -o output-dir      Optional: path of the output directory.
                       If not set, the file will be downloaded in the current
                       directory (for magnets) or torrent file folder (for .torrent)
    -r, --rarest-first Use rarest-first piece selection (better for swarm health)
`, os.Args[0])
	os.Exit(2)
}

func main() {
	var outPath string
	var rarestFirst bool
	flag.Usage = usage
	flag.StringVar(&outPath, "o", "", "")
	flag.BoolVar(&rarestFirst, "r", false, "")
	flag.BoolVar(&rarestFirst, "rarest-first", false, "")
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}
	input := os.Args[len(os.Args)-1]

	opts := &torrent.DownloadOptions{
		RarestFirst: rarestFirst,
	}

	var err error
	if strings.HasPrefix(input, "magnet:") {
		if outPath == "" {
			outPath, _ = os.Getwd()
		}
		err = torrent.DownloadMagnetWithProgress(context.Background(), input, outPath, nil, opts)
	} else {
		err = torrent.DownloadWithProgress(context.Background(), input, outPath, opts)
	}

	if err != nil {
		println(err.Error())
		os.Exit(2)
	}
}
