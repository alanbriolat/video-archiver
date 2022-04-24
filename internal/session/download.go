package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/internal/pubsub"
	"github.com/alanbriolat/video-archiver/internal/sync_"
)

var (
	ErrDownloadClosed = errors.New("download closed")
)

type DownloadID string

func NewDownloadID() DownloadID {
	return DownloadID(generic.Unwrap(uuid.NewRandom()).String())
}

type DownloadStatus string

const (
	DownloadStatusUndefined   DownloadStatus = ""
	DownloadStatusNew         DownloadStatus = "new"
	DownloadStatusMatching    DownloadStatus = "matching"
	DownloadStatusMatched     DownloadStatus = "matched"
	DownloadStatusFetching    DownloadStatus = "fetching"
	DownloadStatusReady       DownloadStatus = "ready"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusComplete    DownloadStatus = "complete"
	DownloadStatusError       DownloadStatus = "error"
)

var runningStatuses = generic.NewSet(
	DownloadStatusMatching,
	DownloadStatusFetching,
	DownloadStatusDownloading,
)

// IsRunning returns true if the status is one where some active process should be updating the download in some way.
func (s DownloadStatus) IsRunning() bool {
	return runningStatuses.Contains(s)
}

// NonRunning returns the closest preceding status where IsRunning is false, which may be the same status if IsRunning
// is already false.
func (s DownloadStatus) NonRunning() DownloadStatus {
	switch s {
	case DownloadStatusMatching:
		return DownloadStatusNew
	case DownloadStatusFetching:
		return DownloadStatusMatched
	case DownloadStatusDownloading:
		return DownloadStatusReady
	default:
		return s
	}
}

type downloadStoredFields struct {
	ID       DownloadID
	URL      string
	SavePath string
	AddedAt  time.Time
	Status   DownloadStatus
	Error    string

	// Data from "match" stage
	Provider string

	// Data from "fetch" stage
	Name string
}

type downloadEphemeralFields struct {
	Progress int
}

type DownloadState struct {
	downloadStoredFields
	downloadEphemeralFields
}

type Download struct {
	DownloadState

	session   *Session
	ctx       context.Context
	ctxCancel context.CancelFunc

	events pubsub.Publisher[Event]

	running      sync_.Event
	stopped      sync_.Event
	complete     sync_.Event
	done         chan struct{}
	startCommand chan struct{}
	stopCommand  chan struct{}
	stateCommand chan chan generic.Result[DownloadState]
}

func newDownload(session *Session, state DownloadState) (*Download, error) {
	ctx, cancel := context.WithCancel(session.ctx)
	// TODO: do some sanity checks on the DownloadState
	d := &Download{
		DownloadState: state,

		session:   session,
		ctx:       ctx,
		ctxCancel: cancel,

		events: pubsub.NewPublisher[Event](),

		done:         make(chan struct{}),
		startCommand: make(chan struct{}),
		stopCommand:  make(chan struct{}),
		stateCommand: make(chan chan generic.Result[DownloadState]),
	}
	go d.run()
	return d, nil
}

func (d *Download) String() string {
	return fmt.Sprintf("Download{ID:\"%s\", URL:\"%s\", Status:\"%s\"", d.ID, d.URL, d.Status)
}

func (d *Download) log() *zap.SugaredLogger {
	return zap.S().Named("download").With("download_id", d.ID)
}
