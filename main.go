package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	const (
		outDescription = "Optional: path of the output file.\nIf not set, the file will be downloaded in the same folder as the torrent file with the name in that file."
	)

	args := os.Args

	if len(args) <= 1 {
		println("Please provide a path for the torrent file")
		return
	}

	torrentPath := args[1]
	_, err := os.Stat(torrentPath)
	if err != nil {
		fmt.Printf("The path %s you provided is invalid: %s\n", torrentPath, err)
		return
	}

	var outPath string
	flag.StringVar(&outPath, "o", "", outDescription)
	flag.Parse()

	err = Download(torrentPath, outPath)
	if err != nil {
		println(err.Error())
		return
	}
}
