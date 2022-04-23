package session

type downloadEvent struct {
	Download *Download
}

type DownloadAdded downloadEvent
type DownloadRemoved downloadEvent
type DownloadStarted downloadEvent
type DownloadStopped downloadEvent
type DownloadUpdated downloadEvent
type DownloadFileComplete struct {
	downloadEvent
	path string
}
