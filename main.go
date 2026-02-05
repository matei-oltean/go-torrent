package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func usage() {
	fmt.Printf(`%s [options] <torrent-file|magnet-link>

    torrent-file       Path of the torrent file
    magnet-link        Magnet link (starting with magnet:)

    -o output-dir      Optional: path of the output directory.
                       If not set, the file will be downloaded in the current
                       directory (for magnets) or torrent file folder (for .torrent)
`, os.Args[0])
	os.Exit(2)
}

func main() {
	var outPath string
	flag.Usage = usage
	flag.StringVar(&outPath, "o", "", "")
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}
	input := os.Args[len(os.Args)-1]

	var err error
	if strings.HasPrefix(input, "magnet:") {
		if outPath == "" {
			outPath, _ = os.Getwd()
		}
		err = DownloadMagnet(input, outPath)
	} else {
		err = Download(input, outPath)
	}

	if err != nil {
		println(err.Error())
		os.Exit(2)
	}
}
