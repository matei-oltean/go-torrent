package main

import (
	"flag"

	"github.com/matei-oltean/go-torrent/client"
)

func main() {
	const (
		torrentDescription = "Required: path of the torrent file."
		outDescription     = "Optional: path of the output file.\nIf not set, the file will be downloaded in the same folder as the torrent file with the name in that file."
	)
	var torrentPath string
	var outPath string

	flag.StringVar(&torrentPath, "f", "", torrentDescription)
	flag.StringVar(&torrentPath, "file", "", torrentDescription)

	flag.StringVar(&outPath, "o", "", outDescription)
	flag.StringVar(&outPath, "output", "", outDescription)

	flag.Parse()

	if torrentPath == "" {
		println("Please provide a path for the torrent file")
		return
	}

	client, err := client.New(torrentPath)
	if err != nil {
		println(err.Error())
		return
	}
	client.Download(outPath)
}
