package provider

import (
	"context"
	"net/url"

	"github.com/alanbriolat/video-archiver/download"
)

type SourceInfo struct {
	ID    string
	Title string
}

type Source interface {
	// URL should return a value such that Provider.MatchURL would give the same Source.
	URL() *url.URL
	// Info should return information about the download if available, or nil if not. Expected to be nil until after a
	// successful call to Recon.
	Info() *SourceInfo
	// Recon should fetch and store additional information about the download, such that Info will return non-nil.
	Recon(context.Context) error
	// Download should fetch the actual video.
	Download(context.Context, *download.DownloadState) error
}
