package main

import (
	"fmt"

	"github.com/matei-oltean/go-torrent/client"
	"github.com/matei-oltean/go-torrent/peer"
	"github.com/matei-oltean/go-torrent/utils"
)

func main() {
	torrentFilePath := "fileutils/testData/debian-10.2.0-amd64-netinst.iso.torrent"
	client, err := client.New(torrentFilePath)
	if err != nil {
		println(err.Error())
	}
	id := utils.ClientID()
	peer, err := peer.New(client.File.Hash, id, client.PeerAddr.PeersAddresses[0])
	if err != nil {
		println(err.Error())
	} else {
		fmt.Printf("%+v\n", peer)
		peer.Conn.Close()
	}
}
