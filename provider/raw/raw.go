package raw

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/util"
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
	// Attempt to extract filename and extension
	filename, err := util.FilenameFromURL(parsedURL)
	if err != nil {
		return nil, err
	}
	extension := path.Ext(filename)
	if extension == "" {
		return nil, fmt.Errorf("no file extension found")
	}
	if !c.Extensions.Contains(extension) {
		return nil, fmt.Errorf("unknown file extension %v", extension)
	}
	res := source{
		url:      s,
		filename: filename,
	}
	return &res, nil
}

func (c Config) Provider() video_archiver.Provider {
	return video_archiver.Provider{
		Name:  "raw",
		Match: c.Match,
	}
}

type source struct {
	url      string
	filename string
}

func (s *source) URL() string {
	return s.url
}

func (s *source) String() string {
	return s.URL()
}

func (s *source) Recon(ctx context.Context) (video_archiver.ResolvedSource, error) {
	return s, nil
}

func (s *source) Download(d video_archiver.Download) error {
	// TODO: handle HTTP client inside Download?
	client := &http.Client{}
	req, err := http.NewRequestWithContext(d.Context(), "GET", s.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	// TODO: check content type headers etc.

	d.AddExpectedBytes(int(resp.ContentLength))
	return d.SaveStream(s.filename, resp.Body)
}

func init() {
	video_archiver.DefaultProviderRegistry.MustAdd(
		NewConfig().Provider().WithPriority(video_archiver.PriorityLowest),
	)
}
