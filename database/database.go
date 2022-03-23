package database

import (
	"embed"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Database struct {
	db *sqlx.DB
}

func NewDatabase(path string) (*Database, error) {
	db, err := sqlx.Connect("sqlite3", path)
	if err != nil {
		return nil, err
	}
	return &Database{db}, nil
}

func (d *Database) Migrate() error {
	fs, err := iofs.New(embedMigrations, "migrations")
	if err != nil {
		return err
	}
	driver, err := sqlite3.WithInstance(d.db.DB, &sqlite3.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", fs, "sqlite3", driver)
	if err != nil {
		return err
	}
	err = m.Up()
	switch err {
	case nil:
	case migrate.ErrNoChange:
	default:
		return err
	}
	return nil
}

func (d *Database) Close() {
	_ = d.db.Close()
}
