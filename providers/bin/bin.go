package bin

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/url"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/generic"
)

var protocols = generic.NewSet("http", "https")

func Match(s string) (video_archiver.Source, error) {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if !protocols.Contains(parsedURL.Scheme) {
		return nil, fmt.Errorf("unknown URL scheme %v", parsedURL.Scheme)
	}
	urlHash := sha1.New()
	generic.Unwrap(urlHash.Write([]byte(s)))
	filename := fmt.Sprintf("%x.bin", urlHash.Sum(nil))
	return &source{url: s, filename: filename}, nil
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
	return d.SaveURL(s.filename, s.url)
}

func init() {
	video_archiver.DefaultProviderRegistry.MustCreatePriority("bin", Match, video_archiver.PriorityLowest)
}
