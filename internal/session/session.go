package session

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/internal/pubsub"
	"github.com/alanbriolat/video-archiver/internal/sync_"
)

type Config struct {
	DefaultSavePath  string
	ProviderRegistry *video_archiver.ProviderRegistry
}

var DefaultConfig = Config{
	DefaultSavePath:  ".",
	ProviderRegistry: &video_archiver.DefaultProviderRegistry,
}

type downloadsByID = map[DownloadID]*Download

type Session struct {
	config    Config
	ctx       context.Context
	ctxCancel context.CancelFunc
	log       *zap.SugaredLogger

	downloads *sync_.RWMutexed[downloadsByID]
	events    pubsub.Publisher[interface{}]
}

func New(config Config, ctx context.Context) (*Session, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{
		config:    config,
		ctx:       ctx,
		ctxCancel: cancel,
		log:       zap.S().Named("session"),

		downloads: sync_.NewRWMutexed(make(downloadsByID)),
	}
	s.events = pubsub.NewPublisher[interface{}]()
	return s, nil
}

func (s *Session) Subscribe() (pubsub.ReceiverCloser[interface{}], error) {
	return s.events.Subscribe()
}

func (s *Session) ListDownloads() []*Download {
	var list []*Download
	_ = s.downloads.RLocked(func(downloads downloadsByID) error {
		list = make([]*Download, 0, len(downloads))
		for _, d := range downloads {
			list = append(list, d)
		}
		return nil
	})
	return list
}

func (s *Session) GetDownload(id DownloadID) (d *Download) {
	_ = s.downloads.RLocked(func(downloads downloadsByID) error {
		d = downloads[id]
		return nil
	})
	return d
}

func (s *Session) Close() {
	s.ctxCancel()
	downloads := s.downloads.Swap(nil)
	var wg sync.WaitGroup
	wg.Add(len(downloads))
	for _, d := range downloads {
		go func(d *Download) {
			d.Close()
			wg.Done()
		}(d)
	}
	wg.Wait()
	s.events.Close()
}
