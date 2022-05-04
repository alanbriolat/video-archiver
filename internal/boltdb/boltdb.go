package boltdb

import (
	"encoding/json"

	"go.etcd.io/bbolt"

	"github.com/alanbriolat/video-archiver/internal/session"
)

var Buckets = struct {
	Metadata  []byte
	Downloads []byte
}{
	Metadata:  []byte("__metadata__"),
	Downloads: []byte("downloads"),
}

var MetadataKeys = struct {
	Version []byte
}{
	Version: []byte("version"),
}

const currentVersion = 1

type Database interface {
	Close() error

	session.Database
}

type database struct {
	*bbolt.DB
}

func New(path string) (_ Database, err error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) (err error) {
		// Ensure buckets exist
		var metadata *bbolt.Bucket
		if metadata, err = tx.CreateBucketIfNotExists(Buckets.Metadata); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(Buckets.Downloads); err != nil {
			return err
		}

		// Get the current version of the database
		var version int
		if versionBytes := metadata.Get(MetadataKeys.Version); versionBytes == nil {
			version = 0
		} else if err = json.Unmarshal(versionBytes, &version); err != nil {
			return err
		}

		// TODO: perform any migration to get to latest version

		// Set the current version of the database
		if versionBytes, err := json.Marshal(currentVersion); err != nil {
			return err
		} else if err = metadata.Put(MetadataKeys.Version, versionBytes); err != nil {
			return err
		}

		return nil
	})
	return &database{db}, nil
}

func (d database) ListDownloads() (downloads []session.DownloadPersistentState, err error) {
	err = d.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(Buckets.Downloads)
		return bucket.ForEach(func(k, v []byte) error {
			var state session.DownloadPersistentState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			} else {
				downloads = append(downloads, state)
				return nil
			}
		})
	})
	if err != nil {
		return nil, err
	} else {
		return downloads, nil
	}
}

func (d database) WriteDownload(state *session.DownloadPersistentState) error {
	if data, err := json.Marshal(state); err != nil {
		return err
	} else {
		err := d.Update(func(tx *bbolt.Tx) error {
			bucket := tx.Bucket(Buckets.Downloads)
			if err := bucket.Put([]byte(state.ID), data); err != nil {
				return err
			}
			return nil
		})
		return err
	}
}

func (d database) DeleteDownload(state *session.DownloadPersistentState) error {
	return d.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(Buckets.Downloads)
		return bucket.Delete([]byte(state.ID))
	})
}
