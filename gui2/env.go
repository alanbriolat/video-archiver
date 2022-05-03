package gui2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/internal/session"
)

type Env interface {
	Context() context.Context
	Logger() *zap.Logger
	ProviderRegistry() *video_archiver.ProviderRegistry
	Session() *session.Session
	Close()
}

type env struct {
	configDir        string
	ctx              context.Context
	log              *zap.Logger
	providerRegistry *video_archiver.ProviderRegistry
	session          *session.Session
}

func (e *env) Context() context.Context {
	return e.ctx
}

func (e *env) Logger() *zap.Logger {
	return e.log
}

func (e *env) ProviderRegistry() *video_archiver.ProviderRegistry {
	return e.providerRegistry
}

func (e *env) Session() *session.Session {
	return e.session
}

func (e *env) Close() {
	e.session.Close()
}

type EnvBuilder interface {
	Build() (Env, error)
	Context(ctx context.Context) EnvBuilder
	Logger(l *zap.Logger) EnvBuilder
	// ConfigDir specifies an exact configuration path to use.
	ConfigDir(path string) EnvBuilder
	// UserConfigDir will use the specified application to generate a configuration path, according to the platform's
	// default user application data path, e.g. ~/.config/{{appName}}.
	UserConfigDir(appName string) EnvBuilder
}

type envBuilder struct {
	env
	makeConfigDir func(*envBuilder) string
}

func NewEnvBuilder() EnvBuilder {
	b := &envBuilder{
		env: env{
			ctx:              context.Background(),
			log:              zap.L(),
			providerRegistry: &video_archiver.DefaultProviderRegistry,
		},
	}
	return b
}

func (b *envBuilder) Build() (_ Env, err error) {
	// Validate the builder configuration
	if b.makeConfigDir == nil {
		return nil, fmt.Errorf("must use ConfigDir() or UserConfigDir()")
	}

	env := b.env

	// Set up configuration path and database
	env.configDir = b.makeConfigDir(b)
	if err = os.MkdirAll(env.configDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create config dir %v: %w", env.configDir, err)
	}

	sessionConfig := session.DefaultConfig
	sessionConfig.ProviderRegistry = env.providerRegistry
	if env.session, err = session.New(sessionConfig, env.ctx); err != nil {
		return nil, err
	}

	return &env, nil
}

func (b *envBuilder) Context(ctx context.Context) EnvBuilder {
	b.ctx = ctx
	return b
}

func (b *envBuilder) Logger(l *zap.Logger) EnvBuilder {
	b.log = l
	return b
}

func (b *envBuilder) ConfigDir(path string) EnvBuilder {
	b.configDir = path
	b.makeConfigDir = func(b *envBuilder) string { return b.configDir }
	return b
}

func (b *envBuilder) UserConfigDir(appName string) EnvBuilder {
	b.configDir = ""
	b.makeConfigDir = func(b *envBuilder) string { return filepath.Join(glib.GetUserConfigDir(), appName) }
	return b
}
