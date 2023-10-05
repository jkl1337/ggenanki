package ggenanki

import (
	"database/sql"
	"time"
)

func Mod(ts *time.Time) int64 {
	return ts.UnixNano() / int64(time.Millisecond)
}

type Card struct {
	ord int
}

func (c *Card) Ord() int {
	return c.ord
}

type cardWriter struct {
	stmt *sql.Stmt
}

func newCardWriter(tx *sql.Tx) (cardWriter, error) {
	stmt, err := tx.Prepare(`INSERT INTO cards VALUES(null, ?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);`)
	if err != nil {
		return cardWriter{}, err
	}
	return cardWriter{
		stmt: stmt,
	}, nil
}

func (cw cardWriter) Close() error {
	return cw.stmt.Close()
}

func (cw cardWriter) WriteCards(cards []Card, ts *time.Time, deckId int, noteId int64) error {
	mod := Mod(ts)
	for _, c := range cards {
		_, err := cw.stmt.Exec(noteId, deckId, c.ord, mod, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, "")
		if err != nil {
			return err
		}
	}
	return nil
}
