package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var b2s map[bool]string = map[bool]string{
	false: "OFFLINE",
	true:  "ONLINE",
}

type SqliteStorage struct {
	db *sql.DB
	mx *sync.Mutex
}

func NewSqliteStorage(path string) (*SqliteStorage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS samples (
    address  TEXT    NOT NULL,
    at       INTEGER NOT NULL,
    rtt      INTEGER NOT NULL,
    PRIMARY KEY (address, at)
);
CREATE TABLE IF NOT EXISTS events (
    address  TEXT    NOT NULL,
    at       INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    state    TEXT    NOT NULL,
	message  TEXT,
    PRIMARY KEY (address, at)
);`); err != nil {
		db.Close()
		return nil, err
	}
	ret := &SqliteStorage{
		db: db,
		mx: &sync.Mutex{},
	}
	return ret, nil
}

func (stg *SqliteStorage) Close() error {
	return stg.db.Close()
}

func (stg *SqliteStorage) Add(sample Sample) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	_, err := stg.db.Exec(`INSERT INTO samples(address, at, rtt) VALUES (?,?,?)`,
		sample.Address, sample.At.UnixMicro(), sample.RttMs)
	return err
}

func (stg *SqliteStorage) ListAddresses() ([]string, error) {
	rows, err := stg.db.Query(`
		SELECT DISTINCT address
		  FROM samples
  		 ORDER BY address`)
	if err != nil {
		return []string{}, err
	}
	ret := []string{}
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return []string{}, err
		}
		ret = append(ret, address)
	}
	return ret, nil
}

func (stg *SqliteStorage) Filter(address string, from time.Time, to time.Time) ([]Sample, error) {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	return []Sample{}, nil
}

func (stg *SqliteStorage) Prune(address string, to time.Time) error {
	return nil
}

func (stg *SqliteStorage) Event(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	_, err := stg.db.Exec(`INSERT INTO events(address, at, duration, state, message) VALUES (?,?,?,?,?)`,
		event.Address, event.At.UnixMicro(), event.DurMs(), b2s[event.Online],
		fmt.Sprintf("%s :: %s WAS %s FOR %d sec", event.At.Format("2006-01-02 15:04:05"), event.Address, b2s[event.Online], event.DurMs()/1000))
	return err
}

func (stg *SqliteStorage) EventOpen(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	_, err := stg.db.Exec(`INSERT INTO events(address, at, online) VALUES (?,?,?)`,
		event.Address, event.At.UnixMicro(), event.Online)
	return err
}

func (stg *SqliteStorage) EventClose(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	_, err := stg.db.Exec(`
		UPDATE events
		   SET duration = ?
		 WHERE address = ?
		   AND at = ?
		`, event.DurMs(), event.Address, event.At)
	return err
}
