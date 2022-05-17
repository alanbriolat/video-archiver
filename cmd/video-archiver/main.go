package main

import (
	"github.com/alanbriolat/video-archiver/gui"
	_ "github.com/alanbriolat/video-archiver/providers"
	_ "github.com/alanbriolat/video-archiver/providers/bin"
)

func main() {
	gui.Main()
}
