package session

type Event interface {
	// The Download this event relates to (nil if not a Download-specific event).
	Download() *Download
}

type downloadEvent struct {
	download *Download
}

func (e downloadEvent) Download() *Download {
	return e.download
}

type DownloadAdded struct {
	downloadEvent
}
type DownloadRemoved struct {
	downloadEvent
}
type DownloadStarted struct {
	downloadEvent
}
type DownloadStopped struct {
	downloadEvent
	Err error
}
type DownloadUpdated struct {
	downloadEvent
	OldState DownloadState
	NewState DownloadState
}
type DownloadFileComplete struct {
	downloadEvent
	Path string
}
