package youtube

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/kkdai/youtube/v2"
	"github.com/schollz/progressbar/v3"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/download"
)

type sourceInfo struct {
	videoDetails *youtube.Video
	videoFormat  *youtube.Format
}

func (i *sourceInfo) ID() string {
	return i.videoDetails.ID
}

func (i *sourceInfo) Title() string {
	return i.videoDetails.Title
}

func (i *sourceInfo) Ext() string {
	mimeType := strings.SplitN(i.videoFormat.MimeType, ";", 2)[0]
	parts := strings.SplitN(mimeType, "/", 2)
	return parts[1]
}

type Source struct {
	videoID string
	info    *sourceInfo
}

func (s *Source) URL() string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", s.videoID)
}

func (s *Source) Info() video_archiver.SourceInfo {
	if s.info == nil {
		return nil
	} else {
		return s.info
	}
}

func (s *Source) Recon(ctx context.Context) error {
	client := youtube.Client{}
	videoDetails, err := client.GetVideoContext(ctx, s.URL())
	if err != nil {
		return err
	}
	// TODO: select "highest" quality
	formats := videoDetails.Formats.WithAudioChannels()
	videoFormat := &formats[0]
	s.info = &sourceInfo{videoDetails, videoFormat}
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

	client := youtube.Client{}
	stream, size, err := client.GetStream(s.info.videoDetails, s.info.videoFormat)
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

func Match(s string) (video_archiver.Source, error) {
	if parsedURL, err := url.Parse(s); err != nil {
		return nil, err
	} else if videoID, err := extractVideoID(parsedURL); err != nil {
		return nil, err
	} else {
		return &Source{videoID: *videoID}, nil
	}
}

func New() video_archiver.Provider {
	return video_archiver.Provider{Name: "youtube", Match: Match}
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
