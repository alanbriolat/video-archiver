package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/internal/boltdb"
	"github.com/alanbriolat/video-archiver/internal/session"
)

type Env interface {
	Context() context.Context
	Logger() *zap.Logger
	ProviderRegistry() *video_archiver.ProviderRegistry
	DB() boltdb.Database
	Session() *session.Session
	Close()
}

type env struct {
	configDir        string
	ctx              context.Context
	log              *zap.Logger
	providerRegistry *video_archiver.ProviderRegistry
	db               boltdb.Database
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

func (e *env) DB() boltdb.Database {
	return e.db
}

func (e *env) Session() *session.Session {
	return e.session
}

func (e *env) Close() {
	e.session.Close()
	if err := e.db.Close(); err != nil {
		e.log.Sugar().Errorf("error closing database: %v", err)
	}
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
	// DatabaseFilename will use the specified filename within the configuration path as the database path (see also
	// ConfigDir and UserConfigDir).
	DatabaseFilename(filename string) EnvBuilder
	// DatabasePath specifies an exact path to the database file.
	DatabasePath(path string) EnvBuilder
}

type envBuilder struct {
	env
	makeConfigDir    func(*envBuilder) string
	makeDatabasePath func(*envBuilder) string
}

func NewEnvBuilder() EnvBuilder {
	b := &envBuilder{
		env: env{
			ctx:              context.Background(),
			log:              zap.L(),
			providerRegistry: &video_archiver.DefaultProviderRegistry,
		},
	}
	b.DatabaseFilename("session.db")
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

	dbPath := b.makeDatabasePath(b)
	if err = os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
		return nil, fmt.Errorf("failed to create database %v: %w", dbPath, err)
	} else if env.db, err = boltdb.New(dbPath); err != nil {
		return nil, fmt.Errorf("failed to create database %v: %w", dbPath, err)
	}

	sessionConfig := session.DefaultConfig
	sessionConfig.Database = env.db
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

func (b *envBuilder) DatabaseFilename(filename string) EnvBuilder {
	b.makeDatabasePath = func(b *envBuilder) string { return filepath.Join(b.makeConfigDir(b), filename) }
	return b
}

func (b *envBuilder) DatabasePath(path string) EnvBuilder {
	b.makeDatabasePath = func(_ *envBuilder) string { return path }
	return b
}
