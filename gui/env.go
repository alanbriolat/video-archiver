package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/database"
)

type Env interface {
	Context() context.Context
	DB() *database.Database
	Logger() *zap.Logger
	ProviderRegistry() *video_archiver.ProviderRegistry
}

type env struct {
	configDir        string
	ctx              context.Context
	db               *database.Database
	log              *zap.Logger
	providerRegistry *video_archiver.ProviderRegistry
}

func (e *env) Context() context.Context {
	return e.ctx
}

func (e *env) DB() *database.Database {
	return e.db
}

func (e *env) Logger() *zap.Logger {
	return e.log
}

func (e *env) ProviderRegistry() *video_archiver.ProviderRegistry {
	return e.providerRegistry
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
	// Database specifies an existing database object to use.
	Database(db *database.Database) EnvBuilder
	// DatabaseFilename will use the specified filename within the configuration path as the database path
	// (see also ConfigDir and UserConfigDir).
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
	b.DatabaseFilename("database.sqlite3")
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
	if env.db == nil {
		dbPath := b.makeDatabasePath(b)
		if err = os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("failed to create database %v: %w", dbPath, err)
		}
		if env.db, err = database.NewDatabase(dbPath, env.log); err != nil {
			return nil, fmt.Errorf("failed to create database: %w", err)
		}
	}
	// TODO: should Migrate() be left to the consumer of the Env?
	if err = env.db.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
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

func (b *envBuilder) Database(db *database.Database) EnvBuilder {
	b.db = db
	b.makeDatabasePath = nil
	return b
}

func (b *envBuilder) DatabaseFilename(filename string) EnvBuilder {
	b.db = nil
	b.makeDatabasePath = func(b *envBuilder) string { return filepath.Join(b.makeConfigDir(b), filename) }
	return b
}

func (b *envBuilder) DatabasePath(path string) EnvBuilder {
	b.db = nil
	b.makeDatabasePath = func(_ *envBuilder) string { return path }
	return b
}
