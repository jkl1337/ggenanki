package ggenanki

import (
	"bytes"
	"io"
	"os"
	"time"
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

type MediaFilter func(path string, f io.Reader) (io.Reader, error)

type MediaEntry struct {
	name string
	path string
	data []byte
}

type MediaMap struct {
	entries []MediaEntry
	i       int
	current *os.File
	filter  MediaFilter
}

// NewMediaMap created a basic MediaFetcher mapping names to paths on the file system.
//noinspection GoUnusedExportedFunction
func NewMediaMap(paths map[string]string, inMemory map[string][]byte, filter MediaFilter) *MediaMap {
	entries := make([]MediaEntry, len(paths)+len(inMemory))
	i := 0
	for k, v := range paths {
		entries[i] = MediaEntry{name: k, path: v}
		i++
	}
	for k, v := range inMemory {
		entries[i] = MediaEntry{name: k, data: v}
		i++
	}

	return &MediaMap{entries: entries, filter: filter}
}

// FIXME this is the wrong approach, should provide a Writer
func (mm *MediaMap) Next() (string, ReaderWithStat, error) {
	if mm.current != nil {
		mm.current.Close()
		mm.current = nil
	}
	if mm.i >= len(mm.entries) {
		return "", nil, nil
	}
	entry := mm.entries[mm.i]
	name, path, data := entry.name, entry.path, entry.data

	var rdr io.Reader
	var modTime time.Time
	if path != "" {
		next, err := os.Open(path)
		if err != nil {
			return "", nil, err
		}
		fi, err := next.Stat()
		if err != nil {
			next.Close()
			return "", nil, err
		}
		rdr = next
		if mm.filter != nil {
			rdr, err = mm.filter(path, rdr)
			if err != nil {
				next.Close()
				return "", nil, err
			}
		}
		mm.current = next
		modTime = fi.ModTime()
	} else {
		rdr = bytes.NewReader(data)
		modTime = time.Now()
	}
	mm.i++
	return name, fileWithInfo{rdr, modTime}, nil
}

func (mm *MediaMap) Close() {
	if mm.current != nil {
		mm.current.Close()
		mm.current = nil
	}
	mm.i = 0
}

type MediaFetcher interface {
	Next() (string, ReaderWithStat, error)
	Close()
}
