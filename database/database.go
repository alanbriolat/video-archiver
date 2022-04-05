package database

import (
	"embed"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"moul.io/zapgorm2"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type RowID = int64

const NullRowID RowID = 0

type Database struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewDatabase(path string, l *zap.Logger) (*Database, error) {
	gormLog := zapgorm2.New(l)
	config := &gorm.Config{
		Logger: gormLog,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	}
	db, err := gorm.Open(sqlite.Open(path), config)
	if err != nil {
		return nil, err
	}
	return &Database{db, l}, nil
}

func (d *Database) Migrate() error {
	d.log.Info("running database migrations")
	fs, err := iofs.New(embedMigrations, "migrations")
	if err != nil {
		return err
	}
	db, err := d.db.DB()
	if err != nil {
		return err
	}
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
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
		d.log.Info("database migration complete")
	case migrate.ErrNoChange:
		d.log.Info("no database migration required")
	default:
		return err
	}
	return nil
}

func (d *Database) Close() {
}

func (d *Database) GetAllCollections() ([]Collection, error) {
	var collections []Collection
	if err := d.db.Find(&collections).Error; err != nil {
		return nil, err
	} else {
		return collections, nil
	}
}

// GetCollectionByID returns (nil, nil) if the error is only that no such row exists.
func (d *Database) GetCollectionByID(id RowID) (*Collection, error) {
	c := Collection{}
	if err := d.db.Take(&c, "rowid = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
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
	if err := d.db.Take(&c, "name = ?", name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
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
	if err := d.db.Create(c).Error; err != nil {
		return err
	} else {
		return nil
	}
}

// UpdateCollection will set all non-ID values in the database row identified by Collection.ID.
func (d *Database) UpdateCollection(c *Collection) error {
	if err := d.db.Save(c).Error; err != nil {
		return err
	} else {
		return nil
	}
}

// DeleteCollection will delete the collection and all its downloads.
func (d *Database) DeleteCollection(id RowID) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Download{}, "collection_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete dowloads: %w", err)
		} else if err := tx.Delete(&Collection{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete collection: %w", err)
		} else {
			return nil
		}
	})
}

func (d *Database) GetDownloadsByCollectionID(id RowID) ([]Download, error) {
	var downloads []Download
	if err := d.db.Find(&downloads, "collection_id = ?", id).Error; err != nil {
		return nil, err
	} else {
		return downloads, nil
	}
}

// InsertDownload will add a new download to the database, overwriting any auto-generated attributes with those from the database.
func (d *Database) InsertDownload(download *Download) error {
	if err := d.db.Create(download).Error; err != nil {
		return err
	} else {
		return nil
	}
}

// UpdateDownload will set all non-ID values in the database row identified by Download.ID.
func (d *Database) UpdateDownload(download *Download) error {
	if err := d.db.Save(download).Error; err != nil {
		return err
	} else {
		return nil
	}
}

// DeleteDownload will delete the download with the specified ID.
func (d *Database) DeleteDownload(id RowID) error {
	return d.db.Delete(&Download{}, id).Error
}

type Collection struct {
	ID        RowID `gorm:"primaryKey;default:(-);->"`
	Name      string
	Path      string
	Downloads []Download `gorm:"foreignKey:CollectionID"`
}

type DownloadState string

func (s *DownloadState) String() string {
	return string(*s)
}

const (
	DOWNLOAD_STATE_NEW DownloadState = "new"
)

type Download struct {
	ID           RowID `gorm:"primaryKey;default:(-);->"`
	CollectionID RowID `gorm:"column:collection_id"`
	URL          string
	Added        time.Time     `gorm:"default:(-);->"`
	State        DownloadState `gorm:"default:(-)"`
}
