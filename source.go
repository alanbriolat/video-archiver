package video_archiver

import (
	"context"

	"github.com/alanbriolat/video-archiver/download"
)

type SourceInfo interface {
	ID() string
	Title() string
}

type Source interface {
	// URL should return the canonical URL for this source. It is assumed that the Provider.Match that created the
	// Source would successfully match this canonical URL.
	URL() string
	// Info should return information about the download if available, or nil if not. Expected to be nil until after a
	// successful call to Recon.
	Info() SourceInfo
	// Recon should fetch and store additional information about the download, such that Info will return non-nil.
	Recon(context.Context) error
	// Download should fetch the actual video.
	Download(context.Context, *download.DownloadState) error
}
