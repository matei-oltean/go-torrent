[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/matei-oltean/go-torrent/Go)](https://github.com/matei-oltean/go-torrent/actions?query=workflow%3AGo)

Torrent client writen in Go currently being written according to the [original specifications](https://www.bittorrent.org/beps/bep_0003.html).

To use:
* Clone the repository with `https://github.com/matei-oltean/go-torrent.git` (or `go get github.com/matei-oltean/go-torrent`)
* Go to the cloned repository (`cd go-torrent`)
* Build it `go build`
* Launch it with `./go-torrent -f path_to_torrent_file`. You can also force the name of the output file by adding `-o path_to_output`; if not supplied, the file will be downloaded in the same folder as the torrent file with the name supplied by the torrent file. Please note that the client only works with single file torrents.

Next steps:
* Clean up the code (package separation, add tests, etc.)
* Add support for multi file torrents
* Add support for magnet links
* Add support for seeding
* Add a GUI