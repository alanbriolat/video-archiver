package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

type DownloadPersistentState struct {
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

type DownloadEphemeralState struct {
	Progress int
}

type DownloadState struct {
	DownloadPersistentState
	DownloadEphemeralState
}

type downloadStage uint8

const (
	downloadStageUndefined  downloadStage = 0
	downloadStageMatched    downloadStage = 1
	downloadStageResolved   downloadStage = 2
	downloadStageDownloaded downloadStage = 3
)

type Download struct {
	state       DownloadState
	targetStage downloadStage
	mu          sync.RWMutex

	session   *Session
	ctx       context.Context
	ctxCancel context.CancelFunc

	events pubsub.Publisher[Event]

	running      sync_.Event
	stopped      sync_.Event
	complete     sync_.Event
	done         chan struct{}
	startCommand chan downloadStage
	stopCommand  chan struct{}
	stateCommand chan chan generic.Result[DownloadState]

	active         sync.WaitGroup
	activeCancel   context.CancelFunc
	activeFinished chan error
}

func newDownload(session *Session, state DownloadState) (*Download, error) {
	ctx, cancel := context.WithCancel(session.ctx)
	// TODO: do some sanity checks on the DownloadState
	d := &Download{
		state: state,

		session:   session,
		ctx:       ctx,
		ctxCancel: cancel,

		events: pubsub.NewPublisher[Event](),

		done:         make(chan struct{}),
		startCommand: make(chan downloadStage),
		stopCommand:  make(chan struct{}),
		stateCommand: make(chan chan generic.Result[DownloadState]),

		// Should only be one active background process, so channel buffer of 1 means it should never wait to exit
		activeFinished: make(chan error, 1),
	}
	// TODO: do some additional state manipulation, e.g. setting Progress and "complete" event if status is complete
	go d.run()
	return d, nil
}

func (d *Download) ID() DownloadID {
	return d.state.ID
}

func (d *Download) String() string {
	return fmt.Sprintf("Download{ID:\"%s\"}", d.state.ID)
}

func (d *Download) log() *zap.SugaredLogger {
	return zap.S().Named(fmt.Sprintf("download/%v", d.state.ID))
}
