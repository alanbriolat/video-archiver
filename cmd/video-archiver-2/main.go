package main

import (
	"github.com/alanbriolat/video-archiver/gui2"
	_ "github.com/alanbriolat/video-archiver/providers"
	_ "github.com/alanbriolat/video-archiver/providers/bin"
)

func main() {
	gui2.Main()
}
