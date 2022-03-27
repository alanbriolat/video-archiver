package raw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/schollz/progressbar/v3"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/download"
	"github.com/alanbriolat/video-archiver/generic"
)

type Config struct {
	Protocols  generic.Set[string]
	Extensions generic.Set[string]
}

func NewConfig() Config {
	return Config{
		Protocols: generic.NewSet(
			"http",
			"https",
		),
		Extensions: generic.NewSet(
			"flv",
			"m4v",
			"mkv",
			"mp4",
			"webm",
		),
	}
}

func (c *Config) Match(s string) (video_archiver.Source, error) {
	// Expect string to be a URL
	parsedURL, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	// Check that scheme/protocol is valid
	if !c.Protocols.Contains(parsedURL.Scheme) {
		return nil, fmt.Errorf("unknown URL scheme %v", parsedURL.Scheme)
	}
	// Attempt to extract file extension
	// TODO: could maybe fall back to doing a HEAD request at Recon() time?
	path := strings.TrimRight(parsedURL.Path, "/")
	pathElements := strings.Split(path, "/")
	filename := pathElements[len(pathElements)-1]
	filenameParts := strings.Split(filename, ".")
	if len(filenameParts) < 2 {
		return nil, fmt.Errorf("no file extension found")
	}
	extension := filenameParts[len(filenameParts)-1]
	if !c.Extensions.Contains(extension) {
		return nil, fmt.Errorf("unknown file extension %v", extension)
	}
	res := source{
		url: s,
		info: &sourceInfo{
			id:    "",
			title: strings.Join(filenameParts[:len(filenameParts)-1], "."),
			ext:   extension,
		},
	}
	return &res, nil
}

func (c Config) Provider() video_archiver.Provider {
	return video_archiver.Provider{
		Name:  "raw",
		Match: c.Match,
	}
}

type sourceInfo struct {
	id    string
	title string
	ext   string
}

func (i *sourceInfo) ID() string {
	return i.id
}

func (i *sourceInfo) Title() string {
	return i.title
}

func (i *sourceInfo) Ext() string {
	return i.ext
}

type source struct {
	url  string
	info *sourceInfo
}

func (s *source) URL() string {
	return s.url
}

func (s *source) Info() video_archiver.SourceInfo {
	return s.info
}

func (s *source) Recon(ctx context.Context) error {
	return nil
}

func (s *source) Download(ctx context.Context, state *download.DownloadState) error {
	if s.info == nil {
		return fmt.Errorf("must call Recon() first")
	}

	tempFile, err := state.CreateTemp("download-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	// TODO: eliminate this code duplication
	bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")
	_, err = io.Copy(io.MultiWriter(tempFile, bar), resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	generic.Unwrap_(
		video_archiver.DefaultProviderRegistry.Add(
			NewConfig().Provider().WithPriority(video_archiver.PriorityLowest),
		),
	)
}
