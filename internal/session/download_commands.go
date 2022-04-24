package session

import "github.com/alanbriolat/video-archiver/generic"

func (d *Download) State() (DownloadState, error) {
	ch := make(chan generic.Result[DownloadState], 1)
	select {
	case d.stateCommand <- ch:
		select {
		case result := <-ch:
			return result.Parts()
		case <-d.ctx.Done():
			return generic.Err[DownloadState](ErrDownloadClosed).Parts()
		}
	case <-d.ctx.Done():
		return generic.Err[DownloadState](ErrDownloadClosed).Parts()
	}
}

func (d *Download) Start() {
	select {
	case d.startCommand <- struct{}{}:
	case <-d.ctx.Done():
	}
}

func (d *Download) Stop() {
	select {
	case d.stopCommand <- struct{}{}:
	case <-d.ctx.Done():
	}
}

func (d *Download) Running() <-chan struct{} {
	return d.running.Wait()
}

func (d *Download) Stopped() <-chan struct{} {
	return d.stopped.Wait()
}

func (d *Download) Complete() <-chan struct{} {
	return d.complete.Wait()
}

// IsComplete returns true if the "complete" flag was set. Useful to check after waiting on Stopped.
func (d *Download) IsComplete() bool {
	return d.complete.IsSet()
}

func (d *Download) Close() {
	d.ctxCancel()
	<-d.done
}

func (d *Download) Done() <-chan struct{} {
	return d.done
}
