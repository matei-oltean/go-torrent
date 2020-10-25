package main

import (
	"flag"
	"fmt"
	"os"
)

func usage() {
	fmt.Printf(`%s [options] torrent-file

    torrent-file       Required: path of the torrent file
    -o output-file     Optional: path of the output file.
                       If not set, the file will be downloaded in the same
                       folder as the torrent file with the name in that file
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
	torrentPath := os.Args[len(os.Args)-1]
	err := Download(torrentPath, outPath)
	if err != nil {
		println(err.Error())
		os.Exit(2)
	}
}
