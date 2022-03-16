package main

import (
	"context"
	"flag"
	"log"
	"net/url"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/download"
	"github.com/alanbriolat/video-archiver/provider/youtube"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	ctx, _ := context.WithCancel(video_archiver.WithLogger(context.Background(), logger))

	source := flag.String("source", "", "URL to the video page")
	target := flag.String("target", "", "Path to save the video at (must be a directory)")
	flag.Parse()

	if len(*source) == 0 || len(*target) == 0 {
		logger.Fatal("usage: download-video --source https://site.com/videos/id --target /path/to/downloads/")
	}
	sourceUrl, err := url.Parse(*source)
	if err != nil {
		logger.Sugar().Fatalf("%v not a valid URL", *source)
	}

	logger.Sugar().Infof("Downloading from %s into %s", sourceUrl, *target)

	provider := youtube.NewProvider()
	dl := provider.MatchURL(sourceUrl)
	if dl == nil {
		logger.Fatal("Source URL didn't match a provider")
	}

	logger.Info("Starting recon...")
	if err := dl.Recon(ctx); err != nil {
		logger.Sugar().Fatalf("Recon failed: %v", err)
	}
	logger.Sugar().Infof("%+v", *dl.Info())

	logger.Info("Starting download...")
	err = download.WithDownloadState(
		func(state *download.DownloadState) error {
			return dl.Download(ctx, state)
		},
		download.WithTargetDir(*target),
	)
	if err != nil {
		logger.Sugar().Fatalf("Download failed: %v", err)
	}
}
