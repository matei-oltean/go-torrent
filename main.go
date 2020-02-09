package main

import (
	"github.com/matei-oltean/go-torrent/client"
)

func main() {
	torrentFilePath := "fileutils/testData/debian-10.2.0-amd64-netinst.iso.torrent"
	client, err := client.New(torrentFilePath)
	if err != nil {
		println(err.Error())
		return
	}
	client.Download("")
}
