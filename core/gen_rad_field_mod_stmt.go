// GENERATED -- DO NOT EDIT
package core

import (
	"fmt"
	"strings"
)

type RadFieldModStmt interface {
	Accept(visitor RadFieldModStmtVisitor)
}
type RadFieldModStmtVisitor interface {
	VisitTruncateRadFieldModStmt(Truncate)
	VisitColorRadFieldModStmt(Color)
}
type Truncate struct {
	TruncToken Token
	Value      Expr
}

func (e Truncate) Accept(visitor RadFieldModStmtVisitor) {
	visitor.VisitTruncateRadFieldModStmt(e)
}
func (e Truncate) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("TruncToken: %v", e.TruncToken))
	parts = append(parts, fmt.Sprintf("Value: %v", e.Value))
	return fmt.Sprintf("Truncate(%s)", strings.Join(parts, ", "))
}

type Color struct {
	ColorToken Token
	ColorValue Expr
	Regex      Expr
}

func (e Color) Accept(visitor RadFieldModStmtVisitor) {
	visitor.VisitColorRadFieldModStmt(e)
}
func (e Color) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("ColorToken: %v", e.ColorToken))
	parts = append(parts, fmt.Sprintf("ColorValue: %v", e.ColorValue))
	parts = append(parts, fmt.Sprintf("Regex: %v", e.Regex))
	return fmt.Sprintf("Color(%s)", strings.Join(parts, ", "))
}
