package youtube

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/kkdai/youtube/v2"
	"github.com/schollz/progressbar/v3"

	"github.com/alanbriolat/video-archiver/download"
	"github.com/alanbriolat/video-archiver/provider"
)

type sourceInfo struct {
	provider.SourceInfo
	videoDetails *youtube.Video
}

type Source struct {
	videoID string
	info    *sourceInfo
}

func (s *Source) URL() *url.URL {
	if videoURL, err := url.Parse(fmt.Sprintf("https://www.youtube.com/watch?v=%s", s.videoID)); err != nil {
		log.Fatalf("failed to parse generated URL: %v", err)
		return nil
	} else {
		return videoURL
	}
}

func (s *Source) Info() *provider.SourceInfo {
	if s.info == nil {
		return nil
	} else {
		return &s.info.SourceInfo
	}
}

func (s *Source) Recon(ctx context.Context) error {
	client := youtube.Client{}
	video, err := client.GetVideoContext(ctx, s.URL().String())
	if err != nil {
		return err
	}
	s.info = &sourceInfo{
		SourceInfo: provider.SourceInfo{
			ID:    video.ID,
			Title: video.Title,
		},
		videoDetails: video,
	}
	return nil
}

func (s *Source) Download(ctx context.Context, state *download.DownloadState) error {
	if s.info == nil {
		return fmt.Errorf("must call Recon() first")
	}

	tempFile, err := state.CreateTemp("download-*")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	formats := s.info.videoDetails.Formats.WithAudioChannels()
	client := youtube.Client{}
	stream, size, err := client.GetStream(s.info.videoDetails, &formats[0])
	if err != nil {
		return err
	}
	defer stream.Close()

	bar := progressbar.DefaultBytes(size, "downloading")
	_, err = io.Copy(io.MultiWriter(tempFile, bar), stream)
	if err != nil {
		return err
	}

	return nil
}

type Provider struct{}

func (p *Provider) Name() string {
	return "youtube"
}

func (p *Provider) MatchURL(url *url.URL) provider.Source {
	if videoID, err := extractVideoID(url); err == nil {
		return newSource(*videoID)
	}
	return nil
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func newSource(videoID string) provider.Source {
	return &Source{videoID: videoID}
}

// Extract video ID from YouTube URL.
//
// Allowed URL formats:
//		http(s?)://(www|m).youtube.com/(watch|details)?v={VIDEO_ID}
//		http(s?)://(www|m).youtube.com/v/{VIDEO_ID}
//		http(s?)://youtu.be/{VIDEO_ID}
func extractVideoID(url *url.URL) (*string, error) {
	var id string
	switch url.Hostname() {
	case "www.youtube.com":
		fallthrough
	case "m.youtube.com":
		if strings.HasPrefix(url.Path, "/v/") {
			id = strings.SplitN(url.Path, "/", 3)[2]
		} else if url.Path == "/watch" || url.Path == "/details" {
			if url.Query().Has("v") {
				id = url.Query().Get("v")
			} else {
				return nil, fmt.Errorf("missing ?v= query parameter")
			}
		}
	case "youtu.be":
		id = strings.Trim(url.Path, "/")
	default:
		return nil, fmt.Errorf("unrecognised hostname")
	}
	if id == "" {
		return nil, fmt.Errorf("could not extract video ID")
	}
	return &id, nil
}
