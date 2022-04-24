package session

import "github.com/alanbriolat/video-archiver/generic"

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
			case ch <- generic.Ok[DownloadState](d.DownloadState):
			case <-d.ctx.Done():
			}
		// Download.Start()
		case <-d.startCommand:
			d.start()
		// Download.Stop()
		case <-d.stopCommand:
			d.stop(nil)
		}
	}
}

func (d *Download) close() {
	// TODO: stop, cleanup, wait on cleanup
	d.stop(nil)
	d.events.Close()
}

func (d *Download) start() {
	if !d.stopped.Clear() {
		// Already running (or being started) so nothing to do
		return
	}
	d.running.Set()
	// Send "started" event, notifying any subscribers
	d.events.Send(DownloadStarted{downloadEvent{d}})
}

func (d *Download) stop(err error) {
	// TODO: do something useful with err
	if !d.running.Clear() {
		// Not running (or already stopping) so nothing to do
		return
	}
	d.stopped.Set()
	// Send "stopped" event, notifying any subscribers
	d.events.Send(DownloadStopped{downloadEvent{d}, err})
}

func (d *Download) updateState(f func(ds *DownloadState)) {
	old := d.DownloadState
	f(&d.DownloadState)
	// TODO: persist changes to downloadStoredFields
	//d.session.db.SaveDownload(&d.downloadStoredFields)
	if d.DownloadState != old {
		d.events.Send(DownloadUpdated{
			downloadEvent: downloadEvent{d},
		})
	}
}
