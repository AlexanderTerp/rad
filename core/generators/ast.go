package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
)

type TypeInfo struct {
	ClassName string
	Fields    string
}

// todo thought on debuggability: ensure *everything* has a Token of some sort, so that we can always point to the source

func main() {
	outputDir := "./core"

	// literal -> STRING | NUMBER | BOOL
	defineAst(outputDir, "Literal", "interface{}", []string{
		"StringLiteral   : StringLiteralToken Value",
		"IntLiteral      : IntLiteralToken Value",
		"FloatLiteral    : FloatLiteralToken Value",
		"BoolLiteral     : BoolLiteralToken Value",
	})

	// arrayLiteral -> "[" ( literal ( "," literal )* )? "]"
	defineAst(outputDir, "ArrayLiteral", "interface{}", []string{
		"StringArrayLiteral   : []StringLiteral Values",
		"IntArrayLiteral      : []IntLiteral Values",
		"FloatArrayLiteral    : []FloatLiteral Values",
		"BoolArrayLiteral     : []BoolLiteral Values",
		"MixedArrayLiteral    : []LiteralOrArray Values",
		"EmptyArrayLiteral    : ", // todo remove and replace with empty MixedArrayLiteral?
	})

	// literalOrArray -> literal | arrayLiteral
	defineAst(outputDir, "LiteralOrArray", "interface{}", []string{
		"LoaLiteral   : Literal Value",
		"LoaArray     : ArrayLiteral Value",
	})

	// expression     -> logic_or
	// logic_or       -> logic_and ( "or" logic_and )*
	// logic_and      -> equality ( "and" equality )*
	// equality       -> comparison ( ( NOT_EQUAL | EQUAL ) comparison )*
	// comparison     -> term ( ( GT | GTE | LT | LTE ) term )*
	// term           -> factor ( ( "-" | "+" ) factor )*
	// factor         -> unary ( ( "/" | "*" ) unary )*
	// unary          -> ( "!" | "-" ) unary | primary
	// primary        -> "(" expression ")" | literalOrArray | arrayExpr | arrayAccess | functionCall | IDENTIFIER
	// arrayAccess    -> IDENTIFIER "[" expression "]"
	// functionCall   -> IDENTIFIER "(" ( ( expression ( "," expression )* )? ( IDENTIFIER "=" expression ( "," IDENTIFIER "=" expression )* )? )? ")"
	defineAst(outputDir, "Expr", "interface{}", []string{
		"ExprLoa           : LiteralOrArray Value",
		"ArrayExpr         : []Expr Values",
		"ArrayAccess       : Expr Array, Expr Index, Token OpenBracketToken",
		"FunctionCall      : Token Function, []Expr Args, int NumExpectedReturnValues", // todo named args
		"Variable          : Token Name",
		"Binary            : Expr Left, Token Operator, Expr Right", // +, -, *, /
		"Logical           : Expr Left, Token Operator, Expr Right", // and, or
		"Grouping          : Expr Value",                            // ( expr )
		"Unary             : Token Operator, Expr Right",            // !, -, +
		"ListComprehension : Expr Expression, Token For, Token Identifier1, *Token Identifier2, Expr Range, *Expr Condition",
	})

	defineAst(outputDir, "Stmt", "", []string{
		"Empty              :",
		"ExprStmt           : Expr Expression",
		"FunctionStmt       : FunctionCall Call",
		"PrimaryAssign      : []Token Identifiers, []*RslType VarTypes, Expr Initializer", // allow []Expr?
		"CompoundAssign     : Token Name, Token Operator, Expr Value",
		"FileHeader         : FilerHeaderToken FhToken",
		"ArgBlock           : Token ArgsKeyword, []ArgStmt Stmts",
		"RadBlock           : Token RadKeyword, RadBlockType RadType, *Expr Source, []RadStmt Stmts",
		"JsonPathAssign     : Token Identifier, JsonPath Path",
		"SwitchBlockStmt    : SwitchBlock Block",
		"SwitchAssignment   : []Token Identifiers, []*RslType VarTypes, SwitchBlock Block",
		"Block			    : []Stmt Stmts",
		"IfStmt             : []IfCase Cases, *Block ElseBlock",
		"IfCase             : Token IfToken, Expr Condition, Block Body",
		"ForStmt			: Token ForToken, Token Identifier1, *Token Identifier2, Expr Range, Block Body",
		"BreakStmt			: Token BreakToken",
		"ContinueStmt		: Token ContinueToken",
	})

	defineAst(outputDir, "ArgStmt", "", []string{
		"ArgDeclaration     : Token Identifier, *Token Rename, *Token Flag, RslType ArgType, " + // todo rename 'Rename'?
			"bool IsOptional, *LiteralOrArray Default, *ArgCommentToken Comment",
	})

	defineAst(outputDir, "RadStmt", "", []string{
		"Fields     : []Token Identifiers",
		"Sort	    : Token SortToken, []Token Identifiers, []SortDir Directions, *SortDir GeneralSort",
		"FieldMods  : []Token Identifiers, []RadFieldModStmt Mods",
	})

	defineAst(outputDir, "RadFieldModStmt", "", []string{
		"Truncate   : Token TruncToken, Expr Value",
		"Color      : Token ColorToken, Expr ColorValue, Expr Regex",
	})

	defineAst(outputDir, "RlaReturning", "[]RuntimeLiteral", []string{
		"SwitchBlock  : Token SwitchToken, *Token Discriminator, []SwitchStmt Stmts",
	})

	defineAst(outputDir, "SwitchStmt", "", []string{
		"SwitchCase     : Token CaseKeyword, []StringLiteral Keys, []Expr Values",
		"SwitchDefault  : Token DefaultKeyword, []Expr Values",
	})
}

func defineAst(outputDir, baseName string, returnType string, types []string) {
	path := fmt.Sprintf("%s/gen_%s.go", outputDir, ToLowerSnakeCase(baseName))
	file, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create file %s: %v\n", path, err)
		os.Exit(1)
	}
	defer file.Close()

	funcMap := template.FuncMap{
		"split": strings.Split,
	}

	tmpl, err := template.New("ast").Funcs(funcMap).Parse(astTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse template: %v\n", err)
		os.Exit(1)
	}

	var typeInfos []TypeInfo
	for _, t := range types {
		parts := strings.Split(t, ":")
		typeInfos = append(typeInfos, TypeInfo{
			ClassName: strings.TrimSpace(parts[0]),
			Fields:    strings.TrimSpace(parts[1]),
		})
	}

	data := struct {
		BaseName   string
		ReturnType string
		Types      []TypeInfo
	}{
		BaseName:   baseName,
		ReturnType: returnType,
		Types:      typeInfos,
	}

	err = tmpl.Execute(file, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not execute template: %v\n", err)
		os.Exit(1)
	}
}

func ToLowerSnakeCase(str string) string {
	pattern := regexp.MustCompile(`([A-Z])`)
	newStr := pattern.ReplaceAllString(str, "_"+strings.ToLower("${1}"))[1:]
	return strings.ToLower(newStr)
}

const astTemplate = `// GENERATED -- DO NOT EDIT
package core
import (
    "fmt"
    "strings"
)
type {{.BaseName}} interface {
    Accept(visitor {{.BaseName}}Visitor){{if .ReturnType}} {{.ReturnType}}{{end}}
}
type {{.BaseName}}Visitor interface {
{{- range .Types }}
    Visit{{.ClassName}}{{$.BaseName}}({{.ClassName}}){{if $.ReturnType}} {{$.ReturnType}}{{end}}
{{- end }}
}
{{- range .Types }}
type {{.ClassName}} struct {
{{- if .Fields }}
{{- $fields := split .Fields ", " }}
{{- range $fields }}
    {{- $parts := split . " " }}
    {{index $parts 1}} {{index $parts 0}}
{{- end }}
{{- end }}
}
func (e {{.ClassName}}) Accept(visitor {{$.BaseName}}Visitor){{if $.ReturnType}} {{$.ReturnType}}{{end}} {
    {{if $.ReturnType}}return {{end}}visitor.Visit{{.ClassName}}{{$.BaseName}}(e)
}
func (e {{.ClassName}}) String() string {
{{- if .Fields }}
    var parts []string
{{- $fields := split .Fields ", " }}
{{- range $fields }}
    {{- $parts := split . " " }}
    parts = append(parts, fmt.Sprintf("{{index $parts 1}}: %v", e.{{index $parts 1}}))
{{- end }}
    return fmt.Sprintf("{{.ClassName}}(%s)", strings.Join(parts, ", "))
{{- else }}
    return "{{.ClassName}}()"
{{- end }}
}
{{- end }}
`
