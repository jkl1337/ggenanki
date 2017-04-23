package ggenanki

import "io"
import (
	"archive/zip"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"os"
	"time"
	"strconv"
	"encoding/json"
)

type fileWithInfo struct {
	io.Reader
	modTime time.Time
}

func (fw fileWithInfo) ModTime() time.Time {
	return fw.modTime
}

type ReaderWithStat interface {
	io.Reader
	ModTime() time.Time
}

type MediaMap struct {
	paths [][]string
	i int
	current *os.File
}

// NewMediaMap created a basic MediaFetcher mapping names to paths on the file system.
func NewMediaMap(paths map[string]string) *MediaMap {
	pathList := make([][]string, len(paths))
	i := 0
	for k, v := range paths {
		pathList[i] = []string{k, v}
		i++
	}
	return &MediaMap{paths: pathList}
}

func (mm *MediaMap) Next() (string, ReaderWithStat, error) {
	if mm.current != nil {
		mm.current.Close()
		mm.current = nil
	}
	if mm.i+1 >= len(mm.paths) {
		return "", nil, nil
	}
	name, path := mm.paths[mm.i][0], mm.paths[mm.i][1]
	next, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	fi, err := next.Stat()
	if err != nil {
		next.Close()
		return "", nil, err
	}
	mm.current = next
	mm.i++
	return name, fileWithInfo{mm.current, fi.ModTime()}, nil
}

func (mm *MediaMap) Close() {
	if mm.current != nil {
		mm.current.Close()
	}
	mm.i = 0
}

type MediaFetcher interface {
	Next() (string, ReaderWithStat, error)
	Close()
}

type Package struct {
	decks []*Deck
}

func NewPackage(decks []*Deck) *Package {
	return &Package{
		decks: decks,
	}
}

func (p *Package) WriteToFile(path string, media MediaFetcher) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := p.Write(f, media); err != nil {
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

func writeMedia(zipw *zip.Writer, media MediaFetcher) error {
	if media == nil {
		f, err := zipw.Create("media")
		if err != nil {
			return err
		}
		_, err = f.Write([]byte("{}"))
		return err
	}

	i := 0
	mediaMap := make(map[string]string)
	for {
		pkgname, file, err := media.Next()
		if err != nil {
			return err
		}
		if file == nil {
			break
		}
		pkgidx := strconv.Itoa(i)
		if err := writeMediaFile(zipw, pkgidx, file); err != nil {
			return err
		}
		mediaMap[pkgidx] = pkgname
		i++
	}

	j, err := json.Marshal(mediaMap)
	if err != nil {
		return err
	}
	f, err := zipw.Create("media")
	if err != nil {
		return err
	}
	_, err = f.Write(j)
	return err
}

func writeMediaFile(zipw *zip.Writer, pkgidx string, file ReaderWithStat) error {
	ch := &zip.FileHeader{Name: pkgidx}
	ch.SetModTime(file.ModTime())
	dst, err := zipw.CreateHeader(ch)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, file)
	return err
}

func (p *Package) Write(w io.Writer, media MediaFetcher) error {
	dbfile, err := ioutil.TempFile("", "genanki-db")
	if err != nil {
		return err
	}
	tmpname := dbfile.Name()
	dbfile.Close()
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
	dbfile, err = os.Open(tmpname)
	if err != nil {
		return err
	}
	defer dbfile.Close()

	if _, err := io.Copy(zdb, dbfile); err != nil {
		return err
	}

	if err := writeMedia(zipw, media); err != nil {
		return err
	}

	return zipw.Close()
}
