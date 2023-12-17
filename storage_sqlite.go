package main

import (
	"database/sql"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var b2s map[bool]string = map[bool]string{
	false: "OFFLINE",
	true:  "ONLINE",
}

var b2s2 map[bool]string = map[bool]string{
	false: "went offline",
	true:  "got online",
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
    at       INTEGER NOT NULL,
    address  TEXT    NOT NULL,
    rtt      INTEGER NOT NULL,
    PRIMARY KEY (at, address)
);
CREATE TABLE IF NOT EXISTS events (
    at       INTEGER NOT NULL,
    address  TEXT    NOT NULL,
    duration INTEGER NOT NULL,
    state    TEXT    NOT NULL,
	message  TEXT,
    PRIMARY KEY (at, address)
);
CREATE TABLE IF NOT EXISTS active (
    address  TEXT    NOT NULL,
    at       INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    state    TEXT    NOT NULL,
	message  TEXT,
    PRIMARY KEY (address)
);
`); err != nil {
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
	t := sample.At.UnixMicro()
	_, err := stg.db.Exec(`INSERT INTO samples(at, address, rtt) VALUES (?,?,?)`,
		t, sample.Address, sample.RttMs)
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

func (stg *SqliteStorage) EventRegister(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	t := event.At.UnixMicro()
	_, err := stg.db.Exec(`
		INSERT INTO events(at, address, duration, state, message)
		VALUES (?,?,?,?,?)
		`, t, event.Address, event.DurMs(), b2s[event.Online], event.String())
	return err
}

func (stg *SqliteStorage) EventOpen(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	t := event.At.UnixMicro()
	if _, err := stg.db.Exec(`
		INSERT OR REPLACE
		INTO active(at, address, duration, state, message)
		VALUES (?,?,?,?,?)
		`, t, event.Address, event.Duration, b2s[event.Online], event.String()); err != nil {
		return err
	}
	_, err := stg.db.Exec(`
		INSERT INTO events(at, address, duration, state, message)
		VALUES (?,?,?,?,?)
		`, t, event.Address, event.Duration, b2s[event.Online], event.String())
	return err
}

func (stg *SqliteStorage) EventUpdate(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	t := event.At.UnixMicro()
	if _, err := stg.db.Exec(`
		INSERT OR REPLACE
		INTO active(at, address, duration, state, message)
		VALUES (?,?,?,?,?)
		`, t, event.Address, event.Duration, b2s[event.Online], event.String()); err != nil {
		return err
	}
	_, err := stg.db.Exec(`
		UPDATE events
		   SET duration = ?,
		       message = ?
		 WHERE address = ?
		   AND at = ?
		`, event.DurMs(), event.String(), event.Address, t)
	return err
}

// the same as Update
func (stg *SqliteStorage) EventClose(event Event) error {
	stg.mx.Lock()
	defer stg.mx.Unlock()
	t := event.At.UnixMicro()
	if _, err := stg.db.Exec(`
		DELETE FROM active WHERE address = ?
		`, event.Address); err != nil {
		return err
	}

	_, err := stg.db.Exec(`
		UPDATE events
		   SET duration = ?,
			   message = ?
		WHERE address = ?
	  	  AND at = ?
		`, event.DurMs(), event.String(), event.Address, t)
	return err
}
