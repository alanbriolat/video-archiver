package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kkdai/youtube/v2"

	"github.com/alanbriolat/video-archiver"
)

type source struct {
	videoID string
}

func (s *source) URL() string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", s.videoID)
}

func (s *source) String() string {
	return s.URL()
}

func (s *source) Recon(ctx context.Context) (video_archiver.ResolvedSource, error) {
	client := youtube.Client{}
	videoDetails, err := client.GetVideoContext(ctx, s.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}
	// TODO: select "highest" quality
	formats := videoDetails.Formats.WithAudioChannels()
	videoFormat := &formats[0]
	return &resolvedSource{
		source:       *s,
		videoDetails: videoDetails,
		videoFormat:  videoFormat,
	}, nil
}

type resolvedSource struct {
	source
	videoDetails *youtube.Video
	videoFormat  *youtube.Format
}

func (s *resolvedSource) Download(d video_archiver.Download) error {
	client := youtube.Client{}
	stream, size, err := client.GetStreamContext(d.Context(), s.videoDetails, s.videoFormat)
	if err != nil {
		return fmt.Errorf("failed to get stream: %w", err)
	}
	defer stream.Close()
	d.AddExpectedBytes(int(size))
	return d.SaveStream(s.getFilename(), stream)
}

func (s *resolvedSource) String() string {
	return fmt.Sprintf("%s [%s]", s.videoDetails.Title, s.videoDetails.ID)
}

func (s *resolvedSource) getFilename() string {
	mimeType := strings.SplitN(s.videoFormat.MimeType, ";", 2)[0]
	ext := strings.SplitN(mimeType, "/", 2)[1]
	return strings.Join([]string{s.videoDetails.Title, s.videoDetails.ID, ext}, ".")
}

func Match(s string) (video_archiver.Source, error) {
	if parsedURL, err := url.Parse(s); err != nil {
		return nil, err
	} else if videoID, err := extractVideoID(parsedURL); err != nil {
		return nil, err
	} else {
		return &source{videoID: *videoID}, nil
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

func init() {
	video_archiver.DefaultProviderRegistry.MustAdd(New())
}
