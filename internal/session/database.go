package session

type Database interface {
	ListDownloads() ([]DownloadPersistentState, error)
	WriteDownload(*DownloadPersistentState) error
	DeleteDownload(*DownloadPersistentState) error
}

type NilDatabase struct{}

func (d NilDatabase) ListDownloads() ([]DownloadPersistentState, error) {
	return nil, nil
}

func (d NilDatabase) WriteDownload(_ *DownloadPersistentState) error {
	return nil
}

func (d NilDatabase) DeleteDownload(_ *DownloadPersistentState) error {
	return nil
}
