package gui

import (
	"context"
	"fmt"
	"sync"

	"github.com/alanbriolat/video-archiver/database"
)

type activeDownloadManager struct {
	sync.Mutex
	app             Application
	activeDownloads map[database.RowID]*activeDownload
}

func newActiveDownloadManager(app Application) *activeDownloadManager {
	return &activeDownloadManager{
		app: app,
	}
}

func (m *activeDownloadManager) StartDownload(d *download, targetStage downloadStage) {
	id := d.GetID()
	m.Lock()
	defer m.Unlock()

	// If already have an active download for this download, then just signal it to keep going
	if active, ok := m.activeDownloads[id]; ok {
		if active.Update(targetStage) {
			m.app.Logger().Debug("updated existing active download")
			return
		} else {
			// If we can't update the active download, it is in the process of exiting and will definitely never
			// modify the download any further, so delete it from the active downloads map and later we'll replace it
			delete(m.activeDownloads, id)
		}
	}

	// If we got this far, we couldn't update a running active download, which means we need to create one
	m.app.Logger().Debug("starting new active download")
	active := newActiveDownload(d, targetStage)
	if m.activeDownloads == nil {
		m.activeDownloads = make(map[database.RowID]*activeDownload)
	}
	m.activeDownloads[id] = active
	// Spawn a goroutine to run the active download
	go func() {
		active.Run(m.app, m.app.Context())
		// Once the active download exits, need to clean it up from the active download manager
		m.Lock()
		defer m.Unlock()
		// Another call to StartDownload may have already replaced this download between it finishing and it deleting
		// itself, so be extra careful to only delete itself.
		if stored, ok := m.activeDownloads[id]; ok && stored == active {
			delete(m.activeDownloads, id)
		}
	}()
}

func (m *activeDownloadManager) StopDownload(d *download, wait bool) {
	id := d.GetID()
	m.Lock()
	defer m.Unlock()

	if active, ok := m.activeDownloads[id]; ok {
		active.Stop()
		if wait {
			<-active.Done()
		}
	}
}

func (m *activeDownloadManager) StopAll() {
	m.Lock()
	defer m.Unlock()

	// Take ownership of all the currently active downloads
	downloads := make([]*activeDownload, 0, len(m.activeDownloads))
	for _, d := range m.activeDownloads {
		downloads = append(downloads, d)
	}
	m.activeDownloads = nil

	for _, d := range downloads {
		d.Stop()
		<-d.Done()
	}
}

// GetDownload will exchange the supplied *download for the corresponding *download from an active download, if one
// exists for that download's ID. Otherwise, just returns the supplied *download.
func (m *activeDownloadManager) GetDownload(d *download) *download {
	id := d.GetID()
	m.Lock()
	defer m.Unlock()

	if active, ok := m.activeDownloads[id]; ok {
		return active.download
	} else {
		return d
	}
}

func (m *activeDownloadManager) IsRunning(d *download) bool {
	id := d.GetID()
	m.Lock()
	defer m.Unlock()

	active, ok := m.activeDownloads[id]
	return ok && active.IsRunning()
}

type downloadStage uint8

const (
	downloadStageUndefined  downloadStage = 0
	downloadStageMatched    downloadStage = 1
	downloadStageResolved   downloadStage = 2
	downloadStageDownloaded downloadStage = 3
)

type activeDownload struct {
	*download
	mu           sync.Mutex
	done         chan struct{}
	cancel       context.CancelFunc
	currentStage downloadStage
	targetStage  downloadStage
}

func newActiveDownload(d *download, targetStage downloadStage) *activeDownload {
	return &activeDownload{
		download:    d,
		done:        make(chan struct{}),
		targetStage: targetStage,
	}
}

func (d *activeDownload) Run(app Application, ctx context.Context) {
	log := d.getLogger(app)
	log.Debugf("active download started")

	d.mu.Lock()
	ctx, d.cancel = context.WithCancel(ctx)

	var err error
	nextStage := d.currentStage

	for {
		// If stage failed, break out of the loop and don't update current stage
		if err != nil {
			log.Errorf("stopping after failure to run stage %v: %v", nextStage, err)
			break
		}
		// Update the current stage, and break out of the loop if we reached our target
		d.currentStage = nextStage
		if d.currentStage >= d.targetStage {
			log.Debugf("reached target stage %v", d.currentStage)
			break
		}
		// Get ready to do the next stage
		nextStage = d.currentStage + 1
		// Allow changes to target stage during long-running stage execution
		d.mu.Unlock()
		// Run the stage, record its outcome
		log.Debugf("running stage %v", nextStage)
		err = d.runStage(app, ctx, nextStage)
		// Acquire the lock again, so start of next loop iteration can examine target vs. current stage
		d.mu.Lock()
	}

	// Close down the active download
	d.cancel()
	d.cancel = nil
	d.targetStage = d.currentStage
	close(d.done)
	d.mu.Unlock()

	log.Debugf("active download stopped")
}

func (d *activeDownload) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
	}
}

// Update sets a new target stage for the download but only if the download is running. If the download was running,
// returns true, otherwise returns false.
func (d *activeDownload) Update(targetStage downloadStage) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.targetStage = targetStage
		return true
	} else {
		return false
	}
}

func (d *activeDownload) Done() <-chan struct{} {
	return d.done
}

func (d *activeDownload) IsRunning() bool {
	select {
	case <-d.done:
		return false
	default:
		return true
	}
}

func (d *activeDownload) runStage(app Application, ctx context.Context, stage downloadStage) error {
	switch stage {
	case downloadStageMatched:
		return d.doMatch(app, ctx)
	case downloadStageResolved:
		return d.doRecon(app, ctx)
	case downloadStageDownloaded:
		return d.doDownload(app, ctx)
	default:
		return fmt.Errorf("invalid target stage %v", stage)
	}
}
