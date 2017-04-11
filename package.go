package ggenanki

import "io"
import (
	"archive/zip"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"os"
	"time"
)

type Package struct {
	decks []*Deck
}

func NewPackage(decks []*Deck) *Package {
	return &Package{
		decks: decks,
	}
}

func (p *Package) WriteToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := p.Write(f); err != nil {
		return err
	}
	return f.Close()
}

func (p *Package) WriteToDb(tx *sql.Tx) error {
	if _, err := tx.Exec(apkgSchema); err != nil {
		return err
	}
	now := time.Now()
	for _, d := range p.decks {
		if err := d.WriteToDb(tx, &now); err != nil {
			return err
		}
	}
	return nil
}

func (p *Package) Write(w io.Writer) error {
	tmpdb, err := ioutil.TempFile("", "genanki-db")
	if err != nil {
		return err
	}
	tmpname := tmpdb.Name()
	tmpdb.Close()
	db, err := sql.Open("sqlite3", tmpname)
	defer os.Remove(tmpname)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := p.WriteToDb(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if err := db.Close(); err != nil {
		return err
	}

	zipw := zip.NewWriter(w)

	zdb, err := zipw.Create("collection.anki2")
	if err != nil {
		return err
	}
	tmpdb, err = os.Open(tmpname)
	if err != nil {
		return err
	}

	if _, err := io.Copy(zdb, tmpdb); err != nil {
		return err
	}

	media, err := zipw.Create("media")
	if err != nil {
		return err
	}
	if _, err := media.Write([]byte("{}")); err != nil {
		return err
	}

	return zipw.Close()
}
