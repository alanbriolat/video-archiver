package main

import (
	"context"
	"flag"
	"log"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/download"
	"github.com/alanbriolat/video-archiver/generic"
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

	logger.Sugar().Infof("Downloading from %s into %s", *source, *target)

	registry := video_archiver.ProviderRegistry{}
	generic.Expect_("error adding provider")(registry.Add(youtube.New()))

	match, err := registry.Match(*source)
	if err != nil {
		logger.Sugar().Fatalf("Match failed: %v", err)
	}

	logger.Info("Starting recon...")
	if err := match.Source.Recon(ctx); err != nil {
		logger.Sugar().Fatalf("Recon failed: %v", err)
	}

	config := video_archiver.NewDownloadConfig()
	targetPath := generic.Unwrap(config.GetTargetPath(match))
	logger.Sugar().Infof("Writing to %v", targetPath)

	logger.Info("Starting download...")
	err = download.WithDownloadState(
		func(state *download.DownloadState) error {
			return match.Source.Download(ctx, state)
		},
		download.WithTargetDir(*target),
	)
	if err != nil {
		logger.Sugar().Fatalf("Download failed: %v", err)
	}
}
