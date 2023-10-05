package ggenanki

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cbroglie/mustache"
)

type Required struct {
	CardOrd           int
	Op                string
	RequiredFieldOrds []int
}

func (r *Required) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{r.CardOrd, r.Op, r.RequiredFieldOrds})
}

type Field struct {
	Name   string   `json:"name"`
	Order  int      `json:"ord"`
	Sticky bool     `json:"sticky"`
	Rtl    bool     `json:"rtl"`
	Font   string   `json:"font"`
	Size   int      `json:"size"`
	Media  []string `json:"media"`
}

type Template struct {
	Name         string      `json:"name"`
	Order        int         `json:"ord"`
	QuestionFmt  string      `json:"qfmt"`
	AnswerFmt    string      `json:"afmt"`
	Did          interface{} `json:"did"`
	BQuestionFmt string      `json:"bqfmt"`
	BAnswerFmt   string      `json:"bafmt"`
}

type ModelData struct {
	Id        int64       `json:"id"`
	Name      string      `json:"name"`
	SortField int         `json:"sortf"`
	DeckId    int         `json:"did"`
	LatexPre  string      `json:"latexPre"`
	LatexPost string      `json:"latexPost"`
	Required  []Required  `json:"req,omitempty"`
	Mod       int64       `json:"mod"`
	Usn       int64       `json:"usn"`
	Type      int         `json:"type"`
	Css       string      `json:"css"`
	Fields    []*Field    `json:"flds"`
	Templates []*Template `json:"tmpls"`
	Vers      []int       `json:"vers"`
}

type Model struct {
	data         ModelData
	clozeFields_ []string
	fieldMap_    map[string]*Field
}

func MakeFields(names ...string) []*Field {
	var ret []*Field = make([]*Field, len(names))
	for i, name := range names {
		ret[i] = NewField(name)
	}
	return ret
}

func NewField(name string) *Field {
	return &Field{
		Name:   name,
		Font:   "Helvetica",
		Media:  []string{},
		Rtl:    false,
		Size:   20,
		Sticky: false,
	}
}

func NewModel(id int64, name string, fields []*Field, templates []*Template, css string, mod int64, cloze bool, d *ModelData) *Model {
	if mod == 0 {
		mod = time.Now().UnixNano() / int64(time.Millisecond)
	}
	m := &Model{
		ModelData{
			Id:        id,
			Name:      name,
			Fields:    fields,
			Templates: templates,
			Css:       css,
			Usn:       -1,
			Mod:       mod,
			Vers:      []int{},
		},
		nil,
		nil,
	}
	if cloze {
		m.data.Type = 1
	}
	if d != nil {
		m.data.SortField = d.SortField
		m.data.LatexPre = d.LatexPre
		m.data.LatexPost = d.LatexPost
		m.data.Usn = d.Usn
		m.data.Type = d.Type
	}
	if m.data.LatexPre == "" {
		m.data.LatexPre = `\documentclass[12pt]{article}
\special{papersize=3in,5in}
\usepackage{amssymb,amsmath}
\pagestyle{empty}
\setlength{\parindent}{0in}
\begin{document}
`
	}
	if m.data.LatexPost == "" {
		m.data.LatexPost = `\end{document}`
	}

	for i, f := range m.data.Fields {
		f.Order = i
	}
	for i, t := range m.data.Templates {
		t.Order = i
	}
	return m
}

var tmplClozeRe1 = regexp.MustCompile(`\{\{[^}]*?cloze:(?:[^}]?:)*(.+?)}}`)
var tmplClozeRe2 = regexp.MustCompile(`<%cloze:(.+?)%>`)

func (m *Model) FieldMap() map[string]*Field {
	if m.fieldMap_ == nil {
		m.fieldMap_ = make(map[string]*Field)
		for _, f := range m.data.Fields {
			m.fieldMap_[f.Name] = f
		}
	}
	return m.fieldMap_
}

func (m *Model) clozeFields() []string {
	fieldMap := m.FieldMap()
	if m.clozeFields_ == nil {
		for _, match := range tmplClozeRe1.FindAllStringSubmatch(m.data.Templates[0].QuestionFmt, -1) {
			if _, ok := fieldMap[match[1]]; ok {
				m.clozeFields_ = append(m.clozeFields_, match[1])
			}
		}
		for _, match := range tmplClozeRe2.FindAllStringSubmatch(m.data.Templates[0].QuestionFmt, -1) {
			if _, ok := fieldMap[match[1]]; ok {
				m.clozeFields_ = append(m.clozeFields_, match[1])
			}
		}
	}
	return m.clozeFields_
}

func (m *Model) required() ([]Required, error) {
	sentinel := "SeNtInEl"
	sentinelMap, emptyMap := map[string]string{}, map[string]string{}
	for _, f := range m.data.Fields {
		sentinelMap[f.Name] = sentinel
		emptyMap[f.Name] = ""
	}

	var ret []Required
	for tord, t := range m.data.Templates {
		var requiredFields []int

		mtmpl, err := mustache.ParseString(t.QuestionFmt)
		if err != nil {
			return nil, err
		}

		for ford, f := range m.data.Fields {
			sentinelMap[f.Name] = ""
			res, err := mtmpl.Render(sentinelMap)
			if err != nil {
				return nil, err
			}
			sentinelMap[f.Name] = sentinel
			if !strings.Contains(res, sentinel) {
				// When this field is missing, there is no other content, so this field is required
				requiredFields = append(requiredFields, ford)
			}
		}

		if len(requiredFields) > 0 {
			ret = append(ret, Required{tord, "all", requiredFields})
			continue
		}

		// no required fields for "all", switch to "any"
		for ford, f := range m.data.Fields {
			emptyMap[f.Name] = sentinel
			res, err := mtmpl.Render(emptyMap)
			if err != nil {
				return nil, err
			}
			emptyMap[f.Name] = ""
			if strings.Contains(res, sentinel) {
				// When this field is present, there is meaningful content in the question
				requiredFields = append(requiredFields, ford)
			}
		}

		if len(requiredFields) == 0 {
			return nil, errors.New(fmt.Sprintf("Could not compute required fields for this template, please chack the formatting of \"qfmt\": %s", t.QuestionFmt))
		}
		ret = append(ret, Required{tord, "any", requiredFields})
	}
	return ret, nil
}

func (m *Model) Id() int64 {
	return m.data.Id
}

func (m *Model) Data(deckId int) (*ModelData, error) {
	if m.data.Required == nil && m.data.Type == 0 {
		var err error
		m.data.Required, err = m.required()
		if err != nil {
			return nil, err
		}
	}
	d := m.data
	d.DeckId = deckId
	return &d, nil
}
