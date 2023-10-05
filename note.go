package ggenanki

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Note struct {
	model     *Model
	fields    []string
	sortField string
	tags      string
	guid      string
	cards     []Card
}

func NewNote(model *Model, fields []string, sortField string, tags string, guid string) *Note {
	return &Note{
		model:     model,
		fields:    fields,
		sortField: sortField,
		tags:      tags,
		guid:      guid,
	}
}

func (n *Note) Guid() string {
	if n.guid == "" {
		n.guid = guidForFields(n.fields)
	}
	return n.guid
}

func (n *Note) SortField() string {
	if n.sortField == "" {
		if len(n.fields) == 0 {
			return ""
		}
		return n.fields[0]
	}
	return n.sortField
}

func (n *Note) AddTag(tag string) {
	n.tags = strings.Join([]string{n.tags, tag}, " ")
}

var b91Enctab = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#$%&()*+,-./:;<=>?@[]^_`{|}~")

func guidForFields(fields []string) string {
	return GenerateGuid([]byte(strings.Join(fields, "__")))
}

func GenerateGuid(d []byte) string {
	sum := sha256.Sum256(d)
	r := binary.BigEndian.Uint64(sum[:8])
	var o []byte
	for {
		if r == 0 {
			break
		}
		o = append(o, b91Enctab[r%91])
		r /= 91
	}
	for l, r := 0, len(o)-1; l < r; l, r = l+1, r-1 {
		o[l], o[r] = o[r], o[l]
	}
	return string(o)
}

func (n *Note) formatFields() string {
	return strings.Join(n.fields, "\x1f")
}

func (n *Note) formatTags() string {
	s := strings.TrimSpace(n.tags)
	return fmt.Sprintf(" %s ", s)
}

func (n *Note) Model() *Model {
	return n.model
}

var fieldClozeRe = regexp.MustCompile(`\{\{c(\d+)::.+?}}`)

func (n *Note) availableClozeOrds(fields []string) []int {
	fmap := n.model.FieldMap()
	clozeSet := map[int]bool{}
	ret := []int{}
	for _, cfn := range n.model.clozeFields() {
		ford := fmap[cfn].Order
		for _, match := range fieldClozeRe.FindAllStringSubmatch(fields[ford], -1) {
			if cord, err := strconv.Atoi(match[1]); err == nil && cord > 0 {
				clozeSet[cord-1] = true
			}
		}
	}
	for k := range clozeSet {
		ret = append(ret, k)
	}
	return ret
}

func WriteNotes(notes []*Note, tx *sql.Tx, ts *time.Time, deckId int) error {
	cw, err := newCardWriter(tx)
	if err != nil {
		return err
	}
	defer cw.Close()

	stmt, err := tx.Prepare(`INSERT INTO notes VALUES(null, ?,?,?,?,?,?,?,?,?,?);`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	mod := Mod(ts)
	for _, note := range notes {
		res, err := stmt.Exec(note.Guid(), note.model.Id(), mod, -1, note.formatTags(),
			note.formatFields(), note.SortField(), 0, 0, "")
		if err != nil {
			return err
		}
		noteId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := cw.WriteCards(note.Cards(), ts, deckId, noteId); err != nil {
			return err
		}
	}
	err = cw.Close()
	if err != nil {
		return err
	}
	return stmt.Close()
}

func (n *Note) Cards() []Card {
	if n.cards == nil {
		if n.model.data.Type == 1 {
			for _, o := range n.availableClozeOrds(n.fields) {
				n.cards = append(n.cards, Card{o})
			}
		} else {
			// FIXME: error propagation and order of operations
			// There is a subtle dependency in calling the API that leads to the
			// assumption that data.Required has already been filled in
			for _, req := range n.model.data.Required {
				have := false
				if req.Op == "any" {
					for _, o := range req.RequiredFieldOrds {
						if n.fields[o] != "" {
							have = true
							break
						}
					}
				} else if req.Op == "all" {
					for _, o := range req.RequiredFieldOrds {
						if n.fields[o] == "" {
							break
						}
						have = true
					}
				}
				if have {
					n.cards = append(n.cards, Card{req.CardOrd})
				}
			}
		}
	}
	return n.cards
}
