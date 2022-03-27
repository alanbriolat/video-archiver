package video_archiver

import (
	"context"
)

type Source interface {
	// URL should return the canonical URL for this source. It is assumed that the Provider.Match that created the
	// Source would successfully match this canonical URL.
	URL() string
	// A String representation of the Source, which might just be the URL, or after Recon (i.e. as a ResolvedSource)
	// might be e.g. the title and ID of a video.
	String() string
	// Recon should fetch and store additional information about the download, such that Info will return non-nil.
	Recon(context.Context) (ResolvedSource, error)
}

// A ResolvedSource is a Source that is ready to Download.
type ResolvedSource interface {
	Source
	// Download should fetch the actual video, using methods on the Download to save the downloaded data.
	Download(Download) error
}
