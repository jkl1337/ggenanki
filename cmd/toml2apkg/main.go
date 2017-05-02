package main

import (
	anki "github.com/jkl1337/ggenanki"
	"github.com/pelletier/go-toml"
	"io"
	"log"
	"strings"
	"strconv"
	"github.com/mkideal/cli"
	"gopkg.in/libgit2/git2go.v25"
	"bytes"
	"fmt"
	"runtime"
)

type modelInfo struct {
	model      *anki.Model
	fieldNames []string
}

func convertFieldName(f string) string {
	f = strings.ToLower(f)
	return strings.Replace(f, " ", "-", -1)
}


type argT struct {
	cli.Helper
	RepoDir string `cli:"*d"`
	BaseRev string `cli:"*b,base"`
	ReleaseRev string `cli:"r,rel"`
}

type tomlModels struct {
	modelMap map[string]modelInfo
}

func loadModelsFromToml(tt *toml.TomlTree) *tomlModels {
	tm := &tomlModels{ modelMap: make(map[string]modelInfo) }

	for _, mm := range tt.Get("models").([]*toml.TomlTree) {
		m := mm.ToMap()
		var fields []*anki.Field
		var fldNames []string
		for _, fi := range m["flds"].([]interface{}) {
			f := fi.(map[string]interface{})

			name := f["name"].(string)
			fldNames = append(fldNames, convertFieldName(name))
			fields = append(fields, &anki.Field{Name: name,
				Media:                            []string{},
				Sticky:                           f["sticky"].(bool),
				Rtl:                              f["rtl"].(bool),
				Order:                            int(f["ord"].(int64)),
				Font:                             f["font"].(string),
				Size:                             int(f["size"].(int64))})
		}
		var tmpls []*anki.Template
		for _, ti := range m["tmpls"].([]interface{}) {
			t := ti.(map[string]interface{})
			tmpls = append(tmpls, &anki.Template{Name: t["name"].(string),
				QuestionFmt:  t["qfmt"].(string),
				AnswerFmt:    t["afmt"].(string),
				BQuestionFmt: t["bqfmt"].(string),
				BAnswerFmt:   t["bafmt"].(string),
				Order:        int(t["ord"].(int64))})
		}
		typ := m["type"].(int64)
		cloze := false
		if typ == 1 {
			cloze = true
		}
		name := m["name"].(string)
		model := anki.NewModel(m["id"].(int64), name, fields, tmpls, m["css"].(string), m["mod"].(int64), cloze, nil)
		tm.modelMap[name] = modelInfo{
			model:      model,
			fieldNames: fldNames,
		}
	}
	return tm
}

type tomlNotes map[string]*anki.Note

func loadNotesFromToml(tm *tomlModels, tt *toml.TomlTree) tomlNotes {
	tn := make(tomlNotes)

	st := tt.Get("notes").([]*toml.TomlTree)

	for _, nt := range st {
		n := nt.ToMap()
		modname := n["model"].(string)
		mi := tm.modelMap[modname]
		var fieldData []string
		for _, fn := range mi.fieldNames {
			var d string
			if fn == "note-id" {
				d = strconv.FormatInt(n[fn].(int64), 10)
			} else {
				d = n[fn].(string)
			}
			fieldData = append(fieldData, d)
		}
		// FIXME: sort field
		guid := n["guid"].(string)
		note := anki.NewNote(mi.model, fieldData, fieldData[0], n["tags"].(string), guid)
		tn[guid] = note
	}
	return tn
}

func (tn tomlNotes) minus(tno tomlNotes) tomlNotes {
	removed := make(tomlNotes)
	for k, v := range tn {
		if _, have := tno[k]; !have {
			removed[k] = v
		}
	}
	return removed
}

func openFiles(repo *git.Repository, rev string) (io.Reader, error) {
	o, err := repo.RevparseSingle(rev)
	if err != nil {
		return nil, err
	}
	t, err := o.AsTree()
	if err != nil {
		return nil, err
	}

	var buffers []io.Reader
	max := t.EntryCount()
	for i := uint64(0); i < max; i++ {
		e := t.EntryByIndex(i)
		if e.Type != git.ObjectBlob || !strings.HasSuffix(e.Name, ".toml") {
			continue
		}
		b, err := repo.LookupBlob(e.Id)
		if err != nil {
			return nil, err
		}
		buffers = append(buffers, bytes.NewBuffer(b.Contents()))
	}
	return io.MultiReader(buffers...), nil
}

func makeDeck(n1 tomlNotes, n2 tomlNotes) error {
	deck := anki.NewDeck(123456, "Generated", "Generated")

	for _, v := range n1 {
		deck.AddNote(v)
	}
	for _, v := range n2 {
		deck.AddNote(v)
	}
	p := anki.NewPackage([]*anki.Deck{deck})
	return p.WriteToFile("generated.apkg", nil)
}

func main() {
	cli.Run(new(argT), func (ctx *cli.Context) error {
		argv := ctx.Argv().(*argT)

		repodir := argv.RepoDir

		repo, err := git.OpenRepository(repodir)
		if err != nil {
			return err
		}

		ding := func (rev string) (tomlNotes, error) {
			br, err := openFiles(repo, rev)
			if err != nil {
				return nil, err
			}
			bt, err := toml.LoadReader(br)
			if err != nil {
				return nil, err
			}
			models := loadModelsFromToml(bt)
			return loadNotesFromToml(models, bt), nil
		}

		baseRev := argv.BaseRev
		bnotes, err := ding(fmt.Sprintf("%s:data", baseRev))
		if err != nil {
			return err
		}

		cnotes, err := ding(fmt.Sprintf("HEAD:data"))
		if err != nil {
			return err
		}

		dnotes := bnotes.minus(cnotes)

		log.Printf("bnotes: %d, cnotes: %d", len(bnotes), len(cnotes))
		for _, v := range dnotes {
			v.AddTag("removed")
		}
		err = makeDeck(cnotes, dnotes)
		m := new(runtime.MemStats)
		runtime.GC()
		runtime.ReadMemStats(m)
		log.Print(m)
		return err
	})
	return
	//fmt.Println(t)
}
