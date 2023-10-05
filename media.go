package ggenanki

import (
	"io"
	"time"
	"os"
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

type MediaMap struct {
	paths [][]string
	i int
	current *os.File
	filter MediaFilter
}

// NewMediaMap created a basic MediaFetcher mapping names to paths on the file system.
func NewMediaMap(paths map[string]string, filter MediaFilter) *MediaMap {
	pathList := make([][]string, len(paths))
	i := 0
	for k, v := range paths {
		pathList[i] = []string{k, v}
		i++
	}
	return &MediaMap{paths: pathList, filter: filter}
}

// FIXME this is the wrong approach, should provide a Writer
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
	var rdr io.Reader = next
	if mm.filter != nil {
		rdr, err = mm.filter(path, rdr)
		if err != nil {
			next.Close()
			return "", nil, err
		}
	}
	mm.current = next
	mm.i++
	return name, fileWithInfo{rdr, fi.ModTime()}, nil
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

