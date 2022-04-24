package session

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/generic"
)

func (d *Download) run() {
	d.stopped.Set()

	for {
		select {
		// Download.Close() (or parent context cancelled)
		case <-d.ctx.Done():
			d.close()
			close(d.done)
			return
		// Download.State()
		case ch := <-d.stateCommand:
			select {
			case ch <- generic.Ok[DownloadState](d.getState()):
			case <-d.ctx.Done():
			}
		// Download.Start()
		case <-d.startCommand:
			d.start()
		// Download.Stop()
		case <-d.stopCommand:
			d.stop(nil)
		// Active download goroutine exiting
		case err := <-d.activeFinished:
			d.stop(err)
		}
	}
}

func (d *Download) close() {
	d.stop(nil)
	d.events.Close()
}

func (d *Download) start() {
	if !d.stopped.Clear() {
		// Already running (or being started) so nothing to do
		return
	}
	// Note: during this period, the download is neither "running" nor "stopped"

	var ctx context.Context
	ctx, d.activeCancel = context.WithCancel(d.ctx)
	d.active.Add(1)
	go func() {
		defer d.active.Done()
		err := d.runInBackground(ctx)
		d.activeFinished <- err
	}()

	// Set the "running" condition, notifying any waiters
	d.running.Set()
	// Send "started" event, notifying any subscribers
	d.events.Send(DownloadStarted{downloadEvent{d}})
}

func (d *Download) stop(err error) {
	if !d.running.Clear() {
		// Not running (or already stopping) so nothing to do
		return
	}
	// Note: during this period, the download is neither "running" nor "stopped"

	// Ensure the active download goroutine has exited
	d.activeCancel()
	d.active.Wait()
	d.activeCancel = nil

	// Set the "stopped" condition, notifying any waiters
	d.stopped.Set()
	// Send "stopped" event, notifying any subscribers
	d.events.Send(DownloadStopped{downloadEvent{d}, err})

	// Record the error, if there was one (and if there was, "updated" event will be sent to subscribers)
	if err != nil {
		d.updateState(func(ds *DownloadState) {
			ds.Error = err.Error()
			ds.Status = DownloadStatusError
		})
	}
}

func (d *Download) getState() DownloadState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state
}

func (d *Download) updateState(f func(ds *DownloadState)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	old := d.state
	f(&d.state)
	if d.state.Status == DownloadStatusComplete {
		d.state.Progress = 100
		d.complete.Set()
	}
	// TODO: persist changes to DownloadPersistentState
	//d.session.db.SaveDownload(&d.DownloadPersistentState)
	if d.state != old {
		d.events.Send(DownloadUpdated{
			downloadEvent: downloadEvent{d},
			OldState:      old,
			NewState:      d.state,
		})
	}
}

func (d *Download) runInBackground(ctx context.Context) error {
	logger := d.log()

	var provider string
	var url string
	var savePath string
	d.updateState(func(ds *DownloadState) {
		provider = ds.Provider
		url = ds.URL
		savePath = ds.SavePath
		ds.Status = DownloadStatusNew
		ds.Error = ""
	})

	d.updateState(func(ds *DownloadState) {
		ds.Status = DownloadStatusMatching
	})
	var match *video_archiver.Match
	var err error
	if provider != "" {
		logger.Debugf("matching using provider: '%v'", provider)
		match, err = d.session.config.ProviderRegistry.MatchWith(provider, url)
	} else {
		logger.Debug("matching with any provider")
		match, err = d.session.config.ProviderRegistry.Match(url)
	}
	if err == nil {
		logger.Debugf("match successful with provider: '%v'", match.ProviderName)
		d.updateState(func(ds *DownloadState) {
			ds.Status = DownloadStatusMatched
			ds.Provider = match.ProviderName
		})
	} else {
		logger.Errorf("failed to match: %v", err)
		return err
	}

	d.updateState(func(ds *DownloadState) {
		ds.Status = DownloadStatusFetching
	})
	logger.Debug("starting recon")
	resolved, err := match.Source.Recon(ctx)
	if err == nil {
		logger.Debug("recon successful")
		d.updateState(func(ds *DownloadState) {
			ds.Status = DownloadStatusReady
			ds.Name = resolved.String()
		})
	} else {
		logger.Errorf("failed to recon: %v", err)
		return err
	}

	prefix := strings.TrimRight(savePath, string(os.PathSeparator)) + string(os.PathSeparator)
	nextUpdate := time.Now()
	builder := video_archiver.NewDownloadBuilder().WithTargetPrefix(prefix).WithContext(ctx).WithProgressCallback(func(downloaded int, expected int) {
		now := time.Now()
		if now.Before(nextUpdate) {
			return
		}
		nextUpdate = now.Add(d.session.config.ProgressUpdateInterval)
		var progress int
		if expected == 0 {
			progress = 0
		} else {
			progress = (downloaded * 100) / expected
		}
		d.updateState(func(ds *DownloadState) {
			ds.Progress = progress
		})
	})
	d.updateState(func(ds *DownloadState) {
		ds.Status = DownloadStatusDownloading
	})
	logger.Debug("starting download")
	err = func() error {
		if download, err := builder.Build(); err != nil {
			logger.Errorf("failed to create download: %v", err)
			return err
		} else if err = resolved.Download(download); err != nil {
			logger.Errorf("failed to download: %v", err)
			return err
		} else {
			logger.Debug("download successful")
			return nil
		}
	}()
	if err == nil {
		logger.Debug("download successful")
		d.updateState(func(ds *DownloadState) {
			ds.Status = DownloadStatusComplete
		})
	} else {
		logger.Errorf("failed to download: %v", err)
		return err
	}

	return nil
}
