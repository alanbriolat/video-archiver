package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type RowID = int64

const NullRowID RowID = 0

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
	log.Println("running database migrations")
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
		log.Println("database migration complete")
	case migrate.ErrNoChange:
		log.Println("no database migration required")
	default:
		return err
	}
	return nil
}

func (d *Database) Close() {
	_ = d.db.Close()
}

func (d *Database) GetAllCollections() ([]Collection, error) {
	var collections []Collection
	if err := d.db.Select(&collections, `SELECT rowid, * FROM collection ORDER BY name`); err != nil {
		return nil, err
	}
	return collections, nil
}

// GetCollectionByID returns (nil, nil) if the error is only that no such row exists.
func (d *Database) GetCollectionByID(id RowID) (*Collection, error) {
	c := Collection{}
	if err := d.db.Get(&c, `SELECT rowid, * FROM collection WHERE rowid = ? LIMIT 1`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return &c, nil
	}
}

// GetCollectionByName returns (nil, nil) if the error is only that no such row exists.
func (d *Database) GetCollectionByName(name string) (*Collection, error) {
	c := Collection{}
	if err := d.db.Get(&c, `SELECT rowid, * FROM collection WHERE name = ? LIMIT 1`, name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return &c, nil
	}
}

// InsertCollection will add a new collection to the database, overwriting Collection.ID with the new row ID.
func (d *Database) InsertCollection(c *Collection) error {
	if res, err := d.db.NamedExec(`INSERT INTO collection (name, path) VALUES (:name, :path)`, c); err != nil {
		return err
	} else if c.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	return nil
}

// UpdateCollection will set all non-ID values in the database row identified by Collection.ID.
func (d *Database) UpdateCollection(c *Collection) error {
	if res, err := d.db.NamedExec(`UPDATE collection SET name = :name, path = :path WHERE rowid = :rowid`, c); err != nil {
		return err
	} else if count, err := res.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return sql.ErrNoRows
	} else {
		return nil
	}
}

// RefreshCollection will reload the collection information from the database.
func (d *Database) RefreshCollection(c *Collection) error {
	return d.db.Get(c, `SELECT * FROM collection WHERE rowid = ?`, c.ID)
}

// DeleteCollection will delete the collection and all its downloads.
func (d *Database) DeleteCollection(id RowID) error {
	tx := d.db.MustBegin()
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM download WHERE collection_id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete downloads: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM collection WHERE rowid = ?`, id); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

func (d *Database) GetDownloadsByCollectionID(id RowID) ([]Download, error) {
	var downloads []Download
	if err := d.db.Select(&downloads, `SELECT rowid, * FROM download WHERE collection_id = ? ORDER BY added DESC`, id); err != nil {
		return nil, err
	}
	return downloads, nil
}

// InsertDownload will add a new download to the database, overwriting any auto-generated attributes with those from the database.
func (d *Database) InsertDownload(download *Download) error {
	if res, err := d.db.NamedExec(`INSERT INTO download (collection_id, url) VALUES (:collection_id, :url)`, download); err != nil {
		return err
	} else if download.ID, err = res.LastInsertId(); err != nil {
		return err
	} else if err = d.RefreshDownload(download); err != nil {
		return err
	}
	return nil
}

// UpdateDownload will set all non-ID values in the database row identified by Download.ID.
func (d *Database) UpdateDownload(download *Download) error {
	if res, err := d.db.NamedExec(`UPDATE download SET url = :url, state = :state WHERE rowid = :rowid`, download); err != nil {
		return err
	} else if count, err := res.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return sql.ErrNoRows
	} else {
		return nil
	}
}

// RefreshDownload will reload the download information from teh database.
func (d *Database) RefreshDownload(download *Download) error {
	return d.db.Get(download, `SELECT * FROM download WHERE rowid = ?`, download.ID)
}

type Collection struct {
	ID   RowID `db:"rowid"`
	Name string
	Path string
}

type DownloadState string

func (s *DownloadState) String() string {
	return string(*s)
}

const (
	DOWNLOAD_STATE_NEW DownloadState = "new"
)

type Download struct {
	ID           RowID `db:"rowid"`
	CollectionID RowID `db:"collection_id"`
	URL          string
	Added        time.Time
	State        DownloadState
}
