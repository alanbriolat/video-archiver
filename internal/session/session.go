package session

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/internal/pubsub"
	"github.com/alanbriolat/video-archiver/internal/sync_"
)

type Config struct {
	DefaultSavePath  string
	Database         Database
	ProviderRegistry *video_archiver.ProviderRegistry
	// Minimum interval between DownloadUpdated events from progress updates.
	ProgressUpdateInterval time.Duration
}

var DefaultConfig = Config{
	DefaultSavePath:        ".",
	Database:               NilDatabase{},
	ProviderRegistry:       &video_archiver.DefaultProviderRegistry,
	ProgressUpdateInterval: 500 * time.Millisecond,
}

type downloadsByID = map[DownloadID]*Download

type Session struct {
	config    Config
	ctx       context.Context
	ctxCancel context.CancelFunc
	log       *zap.SugaredLogger

	downloads *sync_.RWMutexed[downloadsByID]
	events    pubsub.Publisher[Event]
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
	s.events = pubsub.NewPublisher[Event]()
	// Asynchronously load existing downloads from the database; as long as client code does Subscribe before
	// ListDownloads, there's no change that any downloads will be missed.
	go func() {
		for _, state := range generic.Unwrap(s.config.Database.ListDownloads()) {
			ds := DownloadState{DownloadPersistentState: state}
			// TODO: eliminate the unnecessary write-back to the database this causes?
			_ = generic.Unwrap(s.insertDownload(ds))
		}
	}()
	return s, nil
}

func (s *Session) Subscribe() (pubsub.ReceiverCloser[Event], error) {
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
