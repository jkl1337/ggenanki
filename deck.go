package ggenanki

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type Deck struct {
	id     int
	name   string
	desc   string
	notes  []*Note
	models map[int64]*ModelData
}

func NewDeck(id int, name string, desc string) *Deck {
	return &Deck{
		id:     id,
		name:   name,
		desc:   desc,
		models: make(map[int64]*ModelData),
	}
}

func (d *Deck) AddNote(note *Note) {
	d.notes = append(d.notes, note)
}

func (d *Deck) AddModel(model *Model) error {
	data, err := model.Data(d.id)
	if err != nil {
		return err
	}
	d.models[model.Id()] = data
	return nil
}

func (d *Deck) WriteToDb(tx *sql.Tx, ts *time.Time) error {
	for _, note := range d.notes {
		if err := d.AddModel(note.Model()); err != nil {
			return err
		}
	}
	// FIXME: awful!
	jname, err := json.Marshal(d.name)
	if err != nil {
		return err
	}
	jdeckid, err := json.Marshal(d.id)
	if err != nil {
		return err
	}
	jmodels, err := json.Marshal(d.models)
	if err != nil {
		return err
	}
	jdesc, err := json.Marshal(d.desc)
	if err != nil {
		return err
	}
	qry := strings.Replace(apkgCol, "NAME", string(jname), -1)
	qry = strings.Replace(qry, "DECKID", string(jdeckid), -1)
	qry = strings.Replace(qry, "DECKDESC", string(jdesc), -1)

	if _, err := tx.Exec(qry, string(jmodels)); err != nil {
		return err
	}

	return WriteNotes(d.notes, tx, ts, d.id)
}
