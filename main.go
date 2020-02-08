package main

import (
	"fmt"

	"github.com/matei-oltean/go-torrent/fileutils"
	"github.com/matei-oltean/go-torrent/peer"
	"github.com/matei-oltean/go-torrent/utils"
)

func main() {
	torrentFilePath := "fileutils/testData/debian-10.2.0-amd64-netinst.iso.torrent"
	torrentFile, _ := fileutils.OpenTorrent(torrentFilePath)
	id := utils.ClientID()
	var port uint16 = 6882
	trackerURL, _ := torrentFile.GetAnnounceURL(id, port)
	println(trackerURL)
	response, err := fileutils.GetTrackerResponse(trackerURL)
	if err != nil {
		println(err.Error())
	}
	fmt.Printf("%+v\n", response)
	peer, err := peer.New(torrentFile.Hash, id, response.PeersAddresses[0])
	if err != nil {
		println(err.Error())
	} else {
		fmt.Printf("%+v\n", peer)
		peer.Conn.Close()
	}
}
