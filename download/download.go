package download

import (
	"context"
	"log"
	"os"
)

type downloadConfig struct {
	baseTargetDir string
	baseTempDir   string
	ctx           context.Context
}

type DownloadConfigOption func(*downloadConfig)

func WithTargetDir(dir string) DownloadConfigOption {
	return func(c *downloadConfig) {
		c.baseTargetDir = dir
	}
}

func WithTempDir(dir string) DownloadConfigOption {
	return func(c *downloadConfig) {
		c.baseTempDir = dir
	}
}

//func WithContext()

type DownloadState struct {
	config  downloadConfig
	tempDir string
}

func newDownloadState(config downloadConfig) (*DownloadState, error) {
	// Create target directory
	if len(config.baseTargetDir) > 0 {
		if err := os.MkdirAll(config.baseTargetDir, 0755); err != nil {
			return nil, err
		}
	}
	// Create temporary directory
	tempDir, err := os.MkdirTemp(config.baseTempDir, "video-archiver-*")
	if err != nil {
		return nil, err
	}
	state := &DownloadState{
		config:  config,
		tempDir: tempDir,
	}
	return state, nil
}

func (s *DownloadState) close() {
	// Clean up temporary directory
	if err := os.RemoveAll(s.tempDir); err != nil {
		// TODO: logger.Warn?
		log.Printf("Failed to clean up download state: %v", err)
	}
}

func (s *DownloadState) CreateTemp(pattern string) (*os.File, error) {
	return os.CreateTemp(s.tempDir, pattern)
}

func WithDownloadState(f func(state *DownloadState) error, opts ...DownloadConfigOption) error {
	config := downloadConfig{
		baseTargetDir: "",
		baseTempDir:   os.TempDir(),
	}
	for _, opt := range opts {
		opt(&config)
	}
	if state, err := newDownloadState(config); err != nil {
		return err
	} else {
		defer state.close()
		return f(state)
	}

}
