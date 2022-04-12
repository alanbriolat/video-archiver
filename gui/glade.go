package gui

import (
	"embed"

	"github.com/alanbriolat/video-archiver/glade"
)

//go:embed *.glade
var gladeFiles embed.FS

var GladeRepository = glade.NewRepository(gladeFiles.ReadFile)
