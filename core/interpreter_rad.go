package core

import (
	"fmt"
	tblwriter "github.com/amterp/go-tbl"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"regexp"
)

type RadBlockInterpreter struct {
	i          *MainInterpreter
	invocation *radInvocation
}

func NewRadBlockInterpreter(i *MainInterpreter) *RadBlockInterpreter {
	return &RadBlockInterpreter{i: i}
}

func (r RadBlockInterpreter) Run(block RadBlock) {
	var url *string
	if block.Source != nil {
		src := (*block.Source).Accept(r.i) // todo might be a json blob, in the future
		switch coerced := src.(type) {
		case string:
			url = &coerced
		default:
			r.i.error(block.RadKeyword, "URL must be a string")
		}
	}
	r.invocation = &radInvocation{
		ri:               &r,
		block:            block,
		url:              url,
		fieldsToNotPrint: strset.New(),
		colToTruncate:    make(map[string]int64),
	}

	for _, stmt := range block.Stmts {
		stmt.Accept(r)
	}

	if block.RadType == Request {
		for _, field := range r.invocation.fields.Identifiers {
			r.invocation.fieldsToNotPrint.Add(field.GetLexeme())
		}
	}

	r.invocation.execute()
	r.invocation = nil
}

func (r RadBlockInterpreter) VisitFieldsRadStmt(fields Fields) {
	r.invocation.fields = fields
}

func (r RadBlockInterpreter) VisitFieldModsRadStmt(mods FieldMods) {
	modVisitor := fieldModVisitor{
		identifiers: mods.Identifiers,
		invocation:  r.invocation,
	}
	for _, mod := range mods.Mods {
		mod.Accept(modVisitor)
	}
}

func (r RadBlockInterpreter) VisitSortRadStmt(sort Sort) {
	if sort.GeneralSort != nil {
		// depend on the fact that field stmt must be the first thing in the block, and so already visited
		for i, _ := range r.invocation.fields.Identifiers {
			r.invocation.sorting = append(r.invocation.sorting, ColumnSort{ColIdx: i, Dir: *sort.GeneralSort})
		}
		return
	}

	fieldToIdx := make(map[string]int)
	for i, identifier := range r.invocation.fields.Identifiers {
		fieldToIdx[identifier.GetLexeme()] = i
	}

	for i, identifier := range sort.Identifiers {
		dir := sort.Directions[i]
		if idx, ok := fieldToIdx[identifier.GetLexeme()]; !ok {
			r.i.error(r.invocation.block.RadKeyword, fmt.Sprintf("Sort field '%s' not found in fields", identifier.GetLexeme()))
		} else {
			r.invocation.sorting = append(r.invocation.sorting, ColumnSort{ColIdx: idx, Dir: dir})
		}
	}
}

// == radInvocation ==

type radInvocation struct {
	ri               *RadBlockInterpreter
	block            RadBlock
	url              *string
	fields           Fields
	fieldsToNotPrint *strset.Set
	sorting          []ColumnSort
	colToTruncate    map[string]int64
	colToColor       map[string]radColorMod
}

type radColorMod struct {
	color tblwriter.Color
	regex *regexp.Regexp
}

func (r *radInvocation) execute() {
	fields := r.fields.Identifiers

	if fields == nil {
		// todo instead of just printing, return as string and let user decide what to do with it?
		executeRequestPassthrough(r)
		return
	}

	if r.url != nil {
		jsonFields := lo.Map(fields, func(field Token, _ int) JsonFieldVar {
			return r.ri.i.env.GetJsonField(field)
		})

		data, err := RReq.RequestJson(*r.url)
		if err != nil {
			r.error(fmt.Sprintf("Error requesting JSON: %v", err))
		}

		trie := CreateTrie(r.block.RadKeyword, jsonFields)
		trie.TraverseTrie(data)
	}

	headers := lo.FilterMap(fields, func(field Token, _ int) (string, bool) {
		if r.fieldsToNotPrint.Has(field.GetLexeme()) {
			return "", false
		}
		return field.GetLexeme(), true
	})

	if len(headers) == 0 {
		return
	}

	columns := lo.FilterMap(fields, func(field Token, _ int) ([]string, bool) {
		if r.fieldsToNotPrint.Has(field.GetLexeme()) {
			return nil, false
		}
		return ToStringArray(r.ri.i.env.GetByToken(field).GetMixedArray()), true
	})

	tbl := NewTblWriter()

	tbl.SetHeader(headers)
	for i := range columns[0] {
		row := lo.Map(columns, func(column []string, _ int) string {
			return column[i]
		})
		tbl.Append(row)
	}

	tbl.SetSorting(r.sorting)
	tbl.SetTruncation(headers, r.colToTruncate)

	// todo ensure failed requests get nicely printed
	tbl.Render()
}

// When no fields are specified, we'll simply perform the request and print the output.
func executeRequestPassthrough(r *radInvocation) {
	url := r.url
	if url == nil {
		r.error("Bug! URL should've been validated earlier to be present for passthrough rad block")
		panic(UNREACHABLE)
	}

	// execute request, don't expect responses, just print out the response body
	data, err := RReq.Request(*url)
	if err != nil {
		r.error(fmt.Sprintf("Error requesting: %v", err))
	}

	// todo weird to even allow this. if we allow returning the data in the future, maybe it'll make sense. and we
	//  would allow just the request block version?
	if r.block.RadType != Request {
		if len(data) == 0 || data[len(data)-1] != '\n' {
			RP.Print(data + "\n")
		} else {
			RP.Print(data)
		}
	}
}

func (r *radInvocation) error(msg string) {
	r.ri.i.error(r.block.RadKeyword, msg)
}

// == fieldModVisitor ==

type fieldModVisitor struct {
	identifiers []Token
	invocation  *radInvocation
}

func (f fieldModVisitor) VisitTruncateRadFieldModStmt(truncate Truncate) {
	truncLen := truncate.Value.Accept(f.invocation.ri.i)
	switch coerced := truncLen.(type) {
	case int64:
		for _, identifier := range f.identifiers {
			f.invocation.colToTruncate[identifier.GetLexeme()] = coerced
		}
	default:
		f.invocation.ri.i.error(truncate.TruncToken, "Truncate value must be an integer")
	}
}

func (f fieldModVisitor) VisitColorRadFieldModStmt(color Color) {
	colorValue := color.ColorValue.Accept(f.invocation.ri.i)
	switch coerced := colorValue.(type) {
	case string:
		coercedColor, ok := ColorFromString(coerced)
		if !ok {
			f.invocation.ri.i.error(color.ColorToken, fmt.Sprintf("Invalid color value. Allowed: %s", COLORS))
		}
		regex := color.Regex.Accept(f.invocation.ri.i)
		switch coercedRegex := regex.(type) {
		case string:
			regex, err := regexp.Compile(coercedRegex)
			if err == nil {
				f.invocation.ri.i.error(color.ColorToken, fmt.Sprintf("Error compiling regex pattern: %s", err))
			}
			for _, identifier := range f.identifiers {
				f.invocation.colToColor[identifier.GetLexeme()] = radColorMod{color: coercedColor, regex: regex}
			}
		}
	default:
		f.invocation.ri.i.error(color.ColorToken, "Color value must be a string")
	}
}
