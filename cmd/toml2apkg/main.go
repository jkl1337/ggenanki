package main

import (
	anki "github.com/jkl1337/ggenanki"
	"github.com/pelletier/go-toml"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"strconv"
)

type modelInfo struct {
	model      *anki.Model
	fieldNames []string
}

func convertFieldName(f string) string {
	f = strings.ToLower(f)
	return strings.Replace(f, " ", "-", -1)
}

func main() {
	_ = anki.NewPackage(nil)

	paths, err := filepath.Glob("data/*.toml")
	files := []io.Reader{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, f)
	}
	mr := io.MultiReader(files...)
	config, err := toml.LoadReader(mr)
	for _, f := range files {
		f.(*os.File).Close()
	}

	if err != nil {
		log.Fatal(err)
	}

	tm := config.Get("models").([]*toml.TomlTree)

	deck := anki.NewDeck(123456, "Brosencephalon", "Bros")

	modelMap := make(map[string]modelInfo)

	for _, mm := range tm {
		m := mm.ToMap()
		var flds []*anki.Field
		var fldNames []string
		for _, fi := range m["flds"].([]interface{}) {
			f := fi.(map[string]interface{})

			name := f["name"].(string)
			fldNames = append(fldNames, convertFieldName(name))
			flds = append(flds, &anki.Field{Name: name,
				Media:  []string{},
				Sticky: f["sticky"].(bool),
				Rtl:    f["rtl"].(bool),
				Order:  int(f["ord"].(int64)),
				Font:   f["font"].(string),
				Size:   int(f["size"].(int64))})
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
		model := anki.NewModel(m["id"].(int64), name, flds, tmpls, m["css"].(string), m["mod"].(int64), cloze, nil)
		modelMap[name] = modelInfo{
			model:      model,
			fieldNames: fldNames,
		}
	}
	t := config.Get("notes").([]*toml.TomlTree)

	for _, nt := range t {
		n := nt.ToMap()
		mname := n["model"].(string)
		mi := modelMap[mname]
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
		note := anki.NewNote(mi.model, fieldData, fieldData[0], n["tags"].(string), n["guid"].(string))
		deck.AddNote(note)
	}
	p := anki.NewPackage([]*anki.Deck{deck})
	p.WriteToFile("hello.apkg")

	//fmt.Println(t)
}
