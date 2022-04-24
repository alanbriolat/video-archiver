package session

import (
	"errors"

	"github.com/alanbriolat/video-archiver/generic"
)

type AddDownloadOptions struct {
	// Override download save path; if not set (empty), will use the Session's save path.
	SavePath string
}

func (s *Session) AddDownload(url string, opt *AddDownloadOptions) (*Download, error) {
	if opt == nil {
		opt = &AddDownloadOptions{}
	}
	ds := DownloadState{}
	ds.ID = NewDownloadID()
	ds.URL = url
	ds.Status = DownloadStatusNew
	if opt.SavePath != "" {
		ds.SavePath = opt.SavePath
	} else {
		ds.SavePath = s.config.DefaultSavePath
	}
	return s.insertDownload(ds)
}

func (s *Session) insertDownload(ds DownloadState) (*Download, error) {
	id := ds.ID
	d, err := newDownload(s, ds)
	if err != nil {
		return nil, err
	}
	err = s.downloads.Locked(func(downloads downloadsByID) error {
		if _, ok := downloads[id]; ok {
			return errors.New("duplicate download ID")
		} else {
			downloads[id] = d
			return nil
		}
	})
	if err != nil {
		return nil, err
	} else {
		generic.Unwrap_(d.events.AddSubscriber(s.events, false))
		s.events.Send(DownloadAdded{d})
		return d, err
	}
}