package video_archiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

type Download interface {
	// AddDownloadedBytes increases how many bytes have been successfully downloaded so far.
	AddDownloadedBytes(n int)

	// AddExpectedBytes increases how many bytes are expected to be downloaded.
	AddExpectedBytes(n int)

	// Cancel the Download, stopping any in-progress I/O activity.
	Cancel()

	// Close cleans up any resources associated with the Download, including deleting its temporary directory.
	Close() error

	// Context is the cancellable context of this Download.
	Context() context.Context

	CreateFile(filename string) (io.WriteCloser, error)

	// Progress returns the downloaded and expected bytes of the download.
	Progress() (int, int)

	// SaveHTTPRequest will execute the http.Request with Context() and then download the resulting stream like SaveStream.
	SaveHTTPRequest(filename string, req *http.Request) error

	// SaveStream will download the stream to the named file, calling AddDownloadedBytes as necessary.
	SaveStream(filename string, stream io.Reader) error

	// SaveURL will make a GET request to the URL and then download the resulting stream like SaveStream.
	SaveURL(filename string, url string) error

	// TempSaveStream is like SaveStream, but writing to a temporary file.
	//TempSaveStream(filename string, stream io.Reader) error

	// Write will ignore the data but will send the byte count to AddDownloadedBytes. Allows progress tracking using
	// io.MultiWriter (but ensure the Download is the last writer to avoid counting failed writes).
	Write(p []byte) (n int, err error)
}

type download struct {
	ctx              context.Context
	cancel           context.CancelFunc
	progressCallback func(int, int)
	targetPrefix     string
	//tempDir          string
	expectedBytes   int
	downloadedBytes int
}

func (d *download) AddDownloadedBytes(n int) {
	d.downloadedBytes += n
	if d.progressCallback != nil {
		d.progressCallback(d.Progress())
	}
}

func (d *download) AddExpectedBytes(n int) {
	d.expectedBytes += n
	if d.progressCallback != nil {
		d.progressCallback(d.Progress())
	}
}

func (d *download) Cancel() {
	d.cancel()
}

func (d *download) Close() error {
	//if err := os.RemoveAll(d.tempDir); err != nil {
	//	return fmt.Errorf("failed to delete temp dir: %w", err)
	//}
	return nil
}

func (d *download) Context() context.Context {
	return d.ctx
}

func (d *download) CreateFile(filename string) (io.WriteCloser, error) {
	targetPath := d.targetPath(filename)
	targetDir := path.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0775); err != nil {
		return nil, err
	}
	return os.Create(d.targetPath(filename))
}

func (d *download) Progress() (int, int) {
	return d.downloadedBytes, d.expectedBytes
}

func (d *download) SaveHTTPRequest(filename string, req *http.Request) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}
	req = req.WithContext(d.Context())
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	d.AddExpectedBytes(int(resp.ContentLength))
	return d.SaveStream(filename, resp.Body)
}

func (d *download) SaveStream(filename string, stream io.Reader) error {
	f, err := d.CreateFile(filename)
	if err != nil {
		return fmt.Errorf("failed to open target file: %w", err)
	}
	defer f.Close()

	// TODO: how to cancel io.Copy? relies on the stream having a context?
	_, err = io.Copy(io.MultiWriter(f, d), stream)
	if err != nil {
		return fmt.Errorf("failed to save stream: %w", err)
	}
	return nil
}

func (d *download) SaveURL(filename string, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	return d.SaveHTTPRequest(filename, req)
}

//func (d *download) TempSaveStream(pattern string, stream io.Reader) (string, error) {
//	// TODO: sanitise filename
//	//TODO implement me
//	panic("implement me")
//}

func (d *download) Write(p []byte) (n int, err error) {
	n = len(p)
	d.AddDownloadedBytes(n)
	return n, nil
}

func (d *download) targetPath(filename string) string {
	// TODO: sanitise filename
	targetPathBuilder := strings.Builder{}
	targetPathBuilder.WriteString(d.targetPrefix)
	targetPathBuilder.WriteString(filename)
	return targetPathBuilder.String()
}

//func (d *download) tempPath(filename string) string {
//
//}

type DownloadBuilder interface {
	Build() (Download, error)
	WithContext(ctx context.Context) DownloadBuilder
	WithProgressCallback(f func(downloaded int, expected int)) DownloadBuilder
	WithTargetPrefix(prefix string) DownloadBuilder
	//WithTempPath(path string) DownloadBuilder
	//WithTempDirPattern(pattern string) DownloadBuilder
}

type downloadBuilder struct {
	ctx              context.Context
	progressCallback func(int, int)
	targetPrefix     string
	//tempPath         string
	//tempDirPattern   string
}

func NewDownloadBuilder() DownloadBuilder {
	return &downloadBuilder{
		ctx:          context.Background(),
		targetPrefix: "./",
		//tempPath:       os.TempDir(),
		//tempDirPattern: "video-archiver-*",
	}
}

func (b *downloadBuilder) Build() (Download, error) {
	//var err error
	d := download{}
	d.ctx, d.cancel = context.WithCancel(b.ctx)
	d.progressCallback = b.progressCallback
	d.targetPrefix = b.targetPrefix
	//d.tempDir, err = os.MkdirTemp(b.tempPath, b.tempDirPattern)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to create temp dir: %w", err)
	//}
	return &d, nil
}

func (b *downloadBuilder) WithContext(ctx context.Context) DownloadBuilder {
	b.ctx = ctx
	return b
}

func (b *downloadBuilder) WithProgressCallback(f func(int, int)) DownloadBuilder {
	b.progressCallback = f
	return b
}

func (b *downloadBuilder) WithTargetPrefix(prefix string) DownloadBuilder {
	b.targetPrefix = prefix
	return b
}

//func (b *downloadBuilder) WithTempPath(path string) DownloadBuilder {
//	b.tempPath = path
//	return b
//}
//
//func (b *downloadBuilder) WithTempDirPattern(pattern string) DownloadBuilder {
//	b.tempDirPattern = pattern
//	return b
//}
