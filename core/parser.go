package core

import (
	"fmt"
	"github.com/samber/lo"
)

const (
	onlyOneReturnValueAllowed = "Binary operators are only allowed in expressions with one return value"
)

type Parser struct {
	printer             Printer
	tokens              []Token
	next                int
	nestedForBlockLevel int
}

func NewParser(printer Printer, tokens []Token) *Parser {
	return &Parser{
		printer: printer,
		tokens:  tokens,
		next:    0,
	}
}

func (p *Parser) Parse() []Stmt {
	var statements []Stmt

	p.consumeNewlines()
	p.fileHeaderIfPresent(&statements)
	p.consumeNewlines()
	p.argBlockIfPresent(&statements)
	p.consumeNewlines()

	for !p.isAtEnd() {
		s := p.statement()
		if _, ok := s.(*Empty); !ok {
			statements = append(statements, s)
		}
		p.consumeNewlines()
	}
	return statements
}

func (p *Parser) isAtEnd() bool {
	return p.peek().GetType() == EOF
}

func (p *Parser) peekType(tokenType TokenType) bool {
	return p.peek().GetType() == tokenType
}

func (p *Parser) peekTypeSeries(tokenType ...TokenType) bool {
	for i, t := range tokenType {
		token := p.advance()
		if token.GetType() != t {
			p.next -= i + 1
			return false
		}
	}
	p.next -= len(tokenType)
	return true
}

func (p *Parser) peek() Token {
	return p.tokens[p.next]
}

func (p *Parser) peekTwoAhead() Token {
	return p.tokens[p.next+1]
}

func (p *Parser) matchAny(tokenTypes ...TokenType) bool {
	for _, t := range tokenTypes {
		if p.peekType(t) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) advance() Token {
	if !p.isAtEnd() {
		p.next++
	}
	return p.previous()
}

func (p *Parser) rewind() {
	p.next--
}

func (p *Parser) previous() Token {
	return p.tokens[p.next-1]
}

func (p *Parser) consume(tokenType TokenType, errorMessageIfNotMatch string) Token {
	if p.peekType(tokenType) {
		return p.advance()
	}
	p.error(errorMessageIfNotMatch)
	panic(UNREACHABLE)
}

func (p *Parser) tryConsume(tokenType TokenType) (Token, bool) {
	if p.peekType(tokenType) {
		return p.advance(), true
	}
	return nil, false
}

// todo this func is susceptible to pointing at an uninformative token
func (p *Parser) error(message string) {
	currentToken := p.tokens[p.next]
	p.printer.TokenErrorExit(currentToken, message+"\n")
}

func (p *Parser) fileHeaderIfPresent(statements *[]Stmt) {
	if p.matchAny(FILE_HEADER) {
		previous := p.previous()
		fh := previous.(*FilerHeaderToken)
		*statements = append(*statements, &FileHeader{FhToken: *fh})
	}
}

func (p *Parser) argBlockIfPresent(statements *[]Stmt) {
	if p.matchKeyword(ARGS, GLOBAL_KEYWORDS) {
		argsKeyword := p.previous()
		p.consume(COLON, "Expected ':' after 'args'")
		p.consumeNewlines()

		if !p.matchAny(INDENT) {
			return
		}

		p.consumeNewlines()
		argsBlock := ArgBlock{ArgsKeyword: argsKeyword, Stmts: []ArgStmt{}}
		for !p.matchAny(DEDENT) {
			s := p.argStatement()
			argsBlock.Stmts = append(argsBlock.Stmts, s)
			p.consumeNewlines()
		}
		*statements = append(*statements, &argsBlock)
	}
}

// argBlockConstraint       -> argStringRegexConstraint
//
//	| argIntRangeConstraint
//	| argOneWayReq
//	| argMutualExcl
//
// argStringRegexConstraint -> IDENTIFIER ( "," IDENTIFIER )* "not"? "regex" REGEX
// argIntRangeConstraint    -> IDENTIFIER COMPARATORS INT
// argOneWayReq             -> IDENTIFIER "requires" IDENTIFIER
// argMutualExcl            -> "one_of" IDENTIFIER ( "," IDENTIFIER )+
func (p *Parser) argStatement() ArgStmt {
	if p.matchKeyword(ONE_OF, ARGS_BLOCK_KEYWORDS) {
		panic(NOT_IMPLEMENTED)
	}

	identifier := p.consume(IDENTIFIER, "Expected Identifier or keyword")

	if p.peekType(STRING_LITERAL) ||
		p.peekType(IDENTIFIER) ||
		p.peekType(STRING) ||
		p.peekType(INT_LITERAL) ||
		p.peekType(BOOL) {

		return p.argDeclaration(identifier)
	}

	if p.matchKeyword(REQUIRES, ARGS_BLOCK_KEYWORDS) {
		panic(NOT_IMPLEMENTED)
	}

	// todo rest
	panic(NOT_IMPLEMENTED)
}

func (p *Parser) argDeclaration(identifier Token) ArgStmt {
	var renameLiteral Token
	if p.matchAny(STRING_LITERAL) {
		renameLiteral = p.previous()
	}

	var flag Token
	if p.peekTwoAhead().GetType() == IDENTIFIER {
		if p.peekType(IDENTIFIER) {
			// non-int flag
			flag = p.consume(IDENTIFIER, "Expected Flag")
		} else if p.peekType(INT_LITERAL) {
			// int flag
			flag = p.consume(INT_LITERAL, "Expected Flag")
		}
	}

	rslType := p.rslType()
	if rslType.Type == RslArrayT {
		p.error("Mixed-type arrays are not allowed in arg declaration")
	}

	isOptional := false
	var defaultLiteral LiteralOrArray
	if p.matchAny(QUESTION) {
		isOptional = true
	} else if p.matchAny(EQUAL) {
		isOptional = true
		rslTypeEnum := rslType.Type
		defaultLiteralIfPresent, ok := p.literalOrArray(&rslTypeEnum)
		if !ok {
			p.error("Expected default value")
		} else {
			defaultLiteral = defaultLiteralIfPresent
		}
	}

	var argComment *ArgCommentToken
	if p.matchAny(ARG_COMMENT) {
		argComment = p.previous().(*ArgCommentToken)
	}

	return &ArgDeclaration{
		Identifier: identifier,
		Rename:     &renameLiteral,
		Flag:       &flag,
		ArgType:    rslType,
		IsOptional: isOptional,
		Default:    &defaultLiteral,
		Comment:    argComment,
	}
}

func (p *Parser) statement() Stmt {
	if p.matchKeyword(RAD, GLOBAL_KEYWORDS) {
		return p.radBlock(Rad)
	}

	if p.matchKeyword(REQUEST, GLOBAL_KEYWORDS) {
		return p.radBlock(Request)
	}

	if p.matchKeyword(DISPLAY, GLOBAL_KEYWORDS) {
		return p.radBlock(Display)
	}

	if p.peekKeyword(IF, GLOBAL_KEYWORDS) {
		return p.ifStmt()
	}

	if p.peekKeyword(FOR, GLOBAL_KEYWORDS) {
		return p.forStmt()
	}

	if p.peekKeyword(BREAK, GLOBAL_KEYWORDS) {
		if p.nestedForBlockLevel == 0 {
			p.error("Break statement must be inside a for loop")
		}
		return &BreakStmt{BreakToken: p.consumeKeyword(BREAK, GLOBAL_KEYWORDS)}
	}

	if p.peekKeyword(CONTINUE, GLOBAL_KEYWORDS) {
		if p.nestedForBlockLevel == 0 {
			p.error("Continue statement must be inside a for loop")
		}
		return &ContinueStmt{ContinueToken: p.consumeKeyword(CONTINUE, GLOBAL_KEYWORDS)}
	}

	// todo all keywords

	if p.peekTypeSeries(IDENTIFIER, LEFT_PAREN) {
		return p.functionCallStmt()
	}

	return p.assignment()
}

func (p *Parser) radBlock(radType RadBlockType) *RadBlock {
	radToken := p.previous()

	var srcToken *Expr
	if radType == Request || radType == Rad {
		if p.peekType(COLON) {
			p.error(fmt.Sprintf("Expecting url or other source for %v statement", radType))
		}
		expr := p.expr(1)
		srcToken = &expr
	} else {
		p.consume(COLON, fmt.Sprintf("Expecting ':' to immediately follow %q, preceding indented block", radType))
	}

	var radStatements []RadStmt
	if p.peekType(COLON) || p.nextNonNewLineTokenIs(INDENT) { // todo i think this breaks if there's a newline between the colon and indent
		if radType != Display {
			p.consume(COLON, fmt.Sprintf("Expecting ':' to precede indented %v block", radType))
		}
		p.consumeNewlines()
		if !p.matchAny(INDENT) {
			p.error(fmt.Sprintf("Expecting indented contents in %s block", radType))
		}
		p.consumeNewlines()
		for !p.matchAny(DEDENT, EOF) {
			radStatements = append(radStatements, p.radStatement(radType))
			p.consumeNewlines()
		}
	}

	radBlock := &RadBlock{RadKeyword: radToken, RadType: radType, Source: srcToken, Stmts: radStatements}
	p.validateRadBlock(radBlock)
	return radBlock
}

func (p *Parser) radStatement(radType RadBlockType) RadStmt {
	if p.matchKeyword(FIELDS, RAD_BLOCK_KEYWORDS) {
		return p.radFieldsStatement()
	}

	if p.matchKeyword(SORT, RAD_BLOCK_KEYWORDS) {
		if radType == Request {
			// note: maybe we allow this if we add explicit 'display' statements into request blocks?
			p.error("Sort statement is not allowed in a request block")
		}
		return p.radSortStatement()
	}

	identifiers := p.commaSeparatedIdentifiers()
	p.consume(COLON, "Expected ':' to begin field modifier block")
	p.consumeNewlines()
	p.consume(INDENT, "Expected indented block after colon")

	var mods []RadFieldModStmt
	for !p.matchAny(DEDENT, EOF) {
		p.consumeNewlines()
		if p.matchKeyword(TRUNCATE, RAD_BLOCK_KEYWORDS) {
			mods = append(mods, p.truncStmt())
		}
		if p.matchKeyword(COLOR, RAD_BLOCK_KEYWORDS) {
			mods = append(mods, p.colorStmt())
		}
		// todo other field mod stmts
	}
	return &FieldMods{Identifiers: identifiers, Mods: mods}

	// todo modifier
	// todo table fmt
	// todo field fmt
	// todo filtering?
}

func (p *Parser) truncStmt() RadFieldModStmt {
	truncateToken := p.previous()
	return &Truncate{TruncToken: truncateToken, Value: p.expr(1)}
}

func (p *Parser) colorStmt() RadFieldModStmt {
	colorToken := p.previous()
	return &Color{ColorToken: colorToken, ColorValue: p.expr(1), Regex: p.expr(1)}
}

func (p *Parser) validateRadBlock(radBlock *RadBlock) {
	var reorderedStmts []RadStmt
	hasFieldsStmt := false
	var stmtsRequiringFields []string
	for _, stmt := range radBlock.Stmts {
		switch stmt := stmt.(type) {
		case *Fields:
			if hasFieldsStmt {
				p.error(fmt.Sprintf("Only one 'fields' statement is allowed in a %s block", radBlock.RadType))
			}
			hasFieldsStmt = true
			// move field statement to the front, so it gets processed first later
			reorderedStmts = append([]RadStmt{stmt}, reorderedStmts...)
		case *Sort:
			stmtsRequiringFields = append(stmtsRequiringFields, stmt.SortToken.GetLexeme())
			reorderedStmts = append(reorderedStmts, stmt)
		case *FieldMods:
			stmtsRequiringFields = append(stmtsRequiringFields, "field modifiers")
			reorderedStmts = append(reorderedStmts, stmt)
		default:
			p.error(fmt.Sprintf("Bug! Unhandled statement type in rad block: %v", stmt))
		}
	}
	if len(stmtsRequiringFields) > 0 && !hasFieldsStmt {
		p.error(fmt.Sprintf("Missing 'fields' statement required by %v statements: %v",
			radBlock.RadType, stmtsRequiringFields))
	}
}

func (p *Parser) radFieldsStatement() RadStmt {
	var identifiers []Token
	identifiers = append(identifiers, p.identifier())
	for !p.matchAny(NEWLINE, DEDENT, EOF) {
		p.consume(COMMA, "Expected ',' between identifiers")
		identifiers = append(identifiers, p.identifier())
	}
	return &Fields{Identifiers: identifiers}
}

func (p *Parser) radSortStatement() RadStmt {
	sortToken := p.previous()
	asc := Asc
	desc := Desc

	if p.matchAny(NEWLINE) {
		return &Sort{SortToken: sortToken, Identifiers: []Token{}, Directions: []SortDir{}, GeneralSort: &asc}
	}

	nextMatchesAsc := p.matchKeyword(ASC, RAD_BLOCK_KEYWORDS)
	nextMatchesDesc := p.matchKeyword(DESC, RAD_BLOCK_KEYWORDS)
	if nextMatchesAsc || nextMatchesDesc {
		p.consume(NEWLINE, "Expected newline after general sort direction")
		dir := lo.Ternary(nextMatchesAsc, asc, desc)
		return &Sort{SortToken: sortToken, Identifiers: []Token{}, Directions: []SortDir{}, GeneralSort: &dir}
	}

	var identifiers []Token
	var directions []SortDir

	for !p.matchAny(NEWLINE, DEDENT, EOF) {
		identifiers = append(identifiers, p.identifier())
		nextMatchesAsc = p.matchKeyword(ASC, RAD_BLOCK_KEYWORDS)
		nextMatchesDesc = p.matchKeyword(DESC, RAD_BLOCK_KEYWORDS)
		if nextMatchesAsc || nextMatchesDesc {
			dir := lo.Ternary(nextMatchesAsc, asc, desc)
			directions = append(directions, dir)
		} else {
			directions = append(directions, asc)
		}
		if !p.matchAny(NEWLINE) {
			p.consume(COMMA, "Expected ',' between sort fields")
		}
	}
	return &Sort{SortToken: sortToken, Identifiers: identifiers, Directions: directions, GeneralSort: nil}
}

func (p *Parser) commaSeparatedIdentifiers() []Token {
	var identifiers []Token
	identifiers = append(identifiers, p.identifier())
	for p.matchAny(COMMA) {
		identifiers = append(identifiers, p.identifier())
	}
	return identifiers
}

func (p *Parser) ifStmt() IfStmt {
	var cases []IfCase
	cases = append(cases, p.ifCase())
	var elseBlock *Block
	for p.peekKeyword(ELSE, GLOBAL_KEYWORDS) {
		p.consumeKeyword(ELSE, GLOBAL_KEYWORDS)
		if p.peekKeyword(IF, GLOBAL_KEYWORDS) {
			cases = append(cases, p.ifCase())
		} else {
			p.consume(COLON, "Expected ':' after 'else'")
			p.consumeNewlines()
			p.consume(INDENT, "Expected indented block after else")
			block := p.block()
			elseBlock = &block
		}
	}
	return IfStmt{Cases: cases, ElseBlock: elseBlock}
}

func (p *Parser) ifCase() IfCase {
	ifToken := p.consumeKeyword(IF, GLOBAL_KEYWORDS)
	condition := p.expr(1)
	p.consume(COLON, "Expected ':' after if condition")
	p.consumeNewlines()
	p.consume(INDENT, "Expected indented block after if condition")
	block := p.block()
	return IfCase{IfToken: ifToken, Condition: condition, Body: block}
}

func (p *Parser) block() Block {
	var stmts []Stmt
	for !p.matchAny(DEDENT) {
		stmts = append(stmts, p.statement())
		p.consumeNewlines()
	}
	return Block{Stmts: stmts}
}

func (p *Parser) forStmt() ForStmt {
	forToken := p.consumeKeyword(FOR, GLOBAL_KEYWORDS)
	identifier1 := p.consume(IDENTIFIER, "Expected identifier after 'for'")
	var identifier2 *Token
	if p.matchAny(COMMA) {
		i := p.consume(IDENTIFIER, "Expected identifier after ','")
		identifier2 = &i
	}
	p.consumeKeyword(IN, GLOBAL_KEYWORDS)
	rangeExpr := p.expr(1)
	p.consume(COLON, "Expected ':' after range expression")
	p.consumeNewlines()
	p.consume(INDENT, "Expected indented block after for")
	p.nestedForBlockLevel += 1
	block := p.block()
	p.nestedForBlockLevel -= 1
	return ForStmt{ForToken: forToken, Identifier1: identifier1, Identifier2: identifier2, Range: rangeExpr, Body: block}
}

func (p *Parser) functionCallStmt() Stmt {
	functionCall := p.functionCall(NO_NUM_RETURN_VALUES_CONSTRAINT)
	return &FunctionStmt{Call: functionCall}
}

func (p *Parser) assignment() Stmt {
	var identifiers []Token
	var rslTypes []*RslType
	identifiers = append(identifiers, p.identifier())

	if p.matchAny(PLUS_EQUAL, MINUS_EQUAL, STAR_EQUAL, SLASH_EQUAL) {
		return p.compoundAssignment(identifiers[0], p.previous())
	}

	for !p.matchAny(EQUAL) {
		if p.peekType(IDENTIFIER) {
			// try to interpret as a type
			r := p.rslType()
			rslTypes = append(rslTypes, &r)

			if p.matchAny(EQUAL) {
				break
			}
		} else {
			rslTypes = append(rslTypes, nil)
		}

		p.consume(COMMA, "Expected ',' between identifiers")
		identifiers = append(identifiers, p.identifier())
	}
	rslTypes = append(rslTypes, nil) // for the last identifier

	// finished matching left side of equal sign, now parse right side

	if p.matchKeyword(SWITCH, GLOBAL_KEYWORDS) {
		block := p.switchBlock(identifiers)
		return &SwitchAssignment{Identifiers: identifiers, VarTypes: rslTypes, Block: block}
	}

	if len(identifiers) == 1 && p.peekType(JSON_PATH_ELEMENT) {
		if !AllNils(rslTypes) {
			// todo perhaps a bit surprising to users?
			p.error("Json path assignment cannot have an explicit type")
		}
		identifier := identifiers[0]
		return p.jsonPathAssignment(identifier)
	}

	return p.primaryAssignment(identifiers, rslTypes)
}

func (p *Parser) compoundAssignment(identifier Token, operator Token) Stmt {
	expr := p.expr(1)
	switch operator.GetType() {
	case PLUS_EQUAL:
		return &CompoundAssign{Name: identifier, Operator: operator, Value: expr}
	case MINUS_EQUAL:
		return &CompoundAssign{Name: identifier, Operator: operator, Value: expr}
	case STAR_EQUAL:
		return &CompoundAssign{Name: identifier, Operator: operator, Value: expr}
	case SLASH_EQUAL:
		return &CompoundAssign{Name: identifier, Operator: operator, Value: expr}
	default:
		p.error("Invalid compound assignment operator")
		panic(UNREACHABLE)
	}
}

func (p *Parser) jsonPathAssignment(identifier Token) Stmt {
	element := p.consume(JSON_PATH_ELEMENT, "Expected root json path element").(*JsonPathElementToken)
	var brackets Token
	if isArray := p.matchAny(BRACKETS); isArray {
		brackets = p.previous()
	}
	elements := []JsonPathElement{{token: *element, arrayToken: &brackets}}
	for !p.matchAny(NEWLINE) {
		p.consume(DOT, "Expected '.' to separate json field elements")
		element = p.consume(JSON_PATH_ELEMENT, "Expected json path element after '.'").(*JsonPathElementToken)
		if p.matchAny(BRACKETS) {
			brackets = p.previous()
		}
		elements = append(elements, JsonPathElement{token: *element, arrayToken: &brackets})
	}
	return &JsonPathAssign{Identifier: identifier, Path: JsonPath{elements: elements}}
}

func (p *Parser) switchBlock(identifiers []Token) SwitchBlock {
	switchToken := p.previous()
	var discriminator Token
	if !p.matchAny(COLON) {
		discriminator = p.consume(IDENTIFIER, "Expected discriminator or colon after switch")
		p.consume(COLON, "Expected ':' after switch discriminator")
	} else if len(identifiers) == 0 {
		// this is a switch block without assignment
		p.error("Switch assignments must have a discriminator")
	}

	p.consumeNewlinesMin(1)
	p.consume(INDENT, "Expected indented block after switch")

	var stmts []SwitchStmt
	for !p.matchAny(DEDENT) {
		stmts = append(stmts, p.switchStmt(discriminator != nil, len(identifiers)))
		p.consumeNewlines()
	}
	return SwitchBlock{SwitchToken: switchToken, Discriminator: &discriminator, Stmts: stmts}
}

func (p *Parser) switchStmt(hasDiscriminator bool, expectedNumReturnValues int) SwitchStmt {
	if p.matchKeyword(CASE, SWITCH_BLOCK_KEYWORDS) {
		return p.caseStmt(hasDiscriminator, expectedNumReturnValues)
	}

	if p.matchKeyword(DEFAULT, SWITCH_BLOCK_KEYWORDS) {
		return p.switchDefaultStmt(expectedNumReturnValues)
	}

	p.error("Expected 'case' or 'default' in switch block")
	panic(UNREACHABLE)
}

func (p *Parser) caseStmt(hasDiscriminator bool, expectedNumReturnValues int) SwitchStmt {
	var keys []StringLiteral
	if hasDiscriminator {
		keys = append(keys, p.stringLiteral())
		for !p.matchAny(COLON) {
			p.consume(COMMA, "Expected ',' between case keys")
			keys = append(keys, p.stringLiteral())
		}
	} else {
		p.consume(COLON, "Expected ':' after 'case' when no discriminator")
	}

	var values []Expr
	values = append(values, p.expr(expectedNumReturnValues))
	for !p.matchAny(NEWLINE) {
		p.consume(COMMA, "Expected ',' between case values")
		values = append(values, p.expr(expectedNumReturnValues))
	}

	if len(values) != expectedNumReturnValues {
		// todo technically redundant due to expr taking expectedNumReturnValues?
		p.error(fmt.Sprintf("Expected %d return values, got %d", expectedNumReturnValues, len(values)))
	}

	return &SwitchCase{CaseKeyword: p.previous(), Keys: keys, Values: values}
}

func (p *Parser) switchDefaultStmt(expectedNumReturnValues int) SwitchStmt {
	p.consume(COLON, "Expected ':' after 'default'")
	var values []Expr
	values = append(values, p.expr(expectedNumReturnValues))
	for !p.matchAny(NEWLINE) {
		p.consume(COMMA, "Expected ',' between default values")
		values = append(values, p.expr(expectedNumReturnValues))
	}

	if len(values) != expectedNumReturnValues {
		// todo technically redundant due to expr taking expectedNumReturnValues?
		p.error(fmt.Sprintf("Expected %d return values, got %d", expectedNumReturnValues, len(values)))
	}

	return &SwitchDefault{DefaultKeyword: p.previous(), Values: values}
}

func (p *Parser) primaryAssignment(identifiers []Token, expectedType []*RslType) Stmt {
	initializer := p.expr(len(identifiers))
	return &PrimaryAssign{Identifiers: identifiers, VarTypes: expectedType, Initializer: initializer}
}

func (p *Parser) expr(numExpectedReturnValues int) Expr {
	return p.or(numExpectedReturnValues)
}

func (p *Parser) or(numExpectedReturnValues int) Expr {
	expr := p.and(numExpectedReturnValues)

	for p.matchKeyword(OR, ALL_KEYWORDS) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.and(1)
		expr = &Logical{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) and(numExpectedReturnValues int) Expr {
	expr := p.equality(numExpectedReturnValues)

	for p.matchKeyword(AND, ALL_KEYWORDS) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.equality(1)
		expr = &Logical{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) equality(numExpectedReturnValues int) Expr {
	expr := p.comparison(numExpectedReturnValues)

	for p.matchAny(NOT_EQUAL, EQUAL_EQUAL) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.comparison(1)
		expr = &Binary{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) comparison(numExpectedReturnValues int) Expr {
	expr := p.term(numExpectedReturnValues)

	for p.matchAny(GREATER, GREATER_EQUAL, LESS, LESS_EQUAL) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.term(1)
		expr = &Binary{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) term(numExpectedReturnValues int) Expr {
	expr := p.factor(numExpectedReturnValues)

	for p.matchAny(MINUS, PLUS) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.factor(1)
		expr = &Binary{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) factor(numExpectedReturnValues int) Expr {
	expr := p.unary(numExpectedReturnValues)

	for p.matchAny(SLASH, STAR) {
		if numExpectedReturnValues != 1 {
			p.error(onlyOneReturnValueAllowed)
		}
		operator := p.previous()
		right := p.unary(1)
		expr = &Binary{Left: expr, Operator: operator, Right: right}
	}

	return expr
}

func (p *Parser) unary(numExpectedReturnValues int) Expr {
	if p.matchAny(EXCLAMATION, MINUS, PLUS) {
		operator := p.previous()
		right := p.unary(1)
		return &Unary{Operator: operator, Right: right}
	}

	return p.primary(numExpectedReturnValues)
}

func (p *Parser) primary(numExpectedReturnValues int) Expr {
	var expr Expr

	if p.matchAny(LEFT_PAREN) {
		expr = p.expr(1)
		p.consume(RIGHT_PAREN, "Expected ')' after expression")
		expr = &Grouping{Value: expr}
	} else if literal, ok := p.literal(nil); ok {
		return &ExprLoa{Value: &LoaLiteral{Value: literal}}
	} else if arrayExpr, ok := p.arrayExpr(); ok {
		expr = arrayExpr
	} else if p.matchAny(IDENTIFIER) {
		identifier := p.previous()
		// ( after an identifier -> function call
		if p.peekType(LEFT_PAREN) {
			p.rewind()
			expr = p.functionCall(numExpectedReturnValues)
		} else {
			expr = &Variable{Name: identifier}
		}
	} else {
		p.error("Expected expression")
	}

	for p.peekType(LEFT_BRACKET) {
		openBracket := p.advance()
		index := p.expr(1)
		p.consume(RIGHT_BRACKET, "Expected ']' after index expression")
		expr = &ArrayAccess{Array: expr, Index: index, OpenBracketToken: openBracket}
	}

	return expr
}

func (p *Parser) functionCall(numExpectedReturnValues int) FunctionCall {
	function := p.consume(IDENTIFIER, "Expected function name")
	p.consume(LEFT_PAREN, "Expected '(' after function name")
	var args []Expr
	if !p.matchAny(RIGHT_PAREN) {
		args = append(args, p.expr(1))
		for !p.matchAny(RIGHT_PAREN) {
			p.consume(COMMA, "Expected ',' between function arguments")
			args = append(args, p.expr(1))
		}
	}
	return FunctionCall{Function: function, Args: args, NumExpectedReturnValues: numExpectedReturnValues}
}

func (p *Parser) arrayExpr() (Expr, bool) {
	if p.matchAny(BRACKETS) {
		return &ArrayExpr{Values: []Expr{}}, true
	}

	if !p.matchAny(LEFT_BRACKET) {
		return nil, false
	}

	if p.matchAny(RIGHT_BRACKET) {
		// technically redundant as it should be one BRACKETS token, but w/e
		return &ArrayExpr{Values: []Expr{}}, true
	}

	expr := p.expr(1)
	if p.peekKeyword(FOR, GLOBAL_KEYWORDS) {
		return p.listComprehension(expr)
	}

	values := []Expr{expr}
	for !p.matchAny(RIGHT_BRACKET) {
		p.consume(COMMA, "Expected ',' between array elements")
		values = append(values, p.expr(1))
	}

	return &ArrayExpr{Values: values}, true
}

func (p *Parser) listComprehension(expr Expr) (Expr, bool) {
	forToken := p.consumeKeyword(FOR, GLOBAL_KEYWORDS)
	identifier1 := p.consume(IDENTIFIER, "Expected identifier after 'for'")
	var identifier2 *Token
	if p.matchAny(COMMA) {
		i := p.consume(IDENTIFIER, "Expected identifier after ','")
		identifier2 = &i
	}
	p.consumeKeyword(IN, GLOBAL_KEYWORDS)
	rangeExpr := p.expr(1)

	if p.matchKeyword(IF, GLOBAL_KEYWORDS) {
		condition := p.expr(1)
		p.consume(RIGHT_BRACKET, "Expected ']' after list comprehension")
		return &ListComprehension{Expression: expr, For: forToken, Identifier1: identifier1, Identifier2: identifier2, Range: rangeExpr, Condition: &condition}, true
	}

	p.consume(RIGHT_BRACKET, "Expected ']' after list comprehension")
	return &ListComprehension{Expression: expr, For: forToken, Identifier1: identifier1, Identifier2: identifier2, Range: rangeExpr, Condition: nil}, true
}

func (p *Parser) literalOrArray(expectedType *RslTypeEnum) (LiteralOrArray, bool) {
	if literal, ok := p.literal(expectedType); ok {
		return &LoaLiteral{Value: literal}, true
	}

	arrayLiteral, ok := p.arrayLiteral(expectedType)
	if ok {
		return &LoaArray{Value: arrayLiteral}, true
	}

	return nil, false
}

func (p *Parser) literal(expectedType *RslTypeEnum) (Literal, bool) {
	if p.peekType(STRING_LITERAL) {
		if expectedType != nil && *expectedType != RslStringT {
			p.error("Expected string literal")
		}
		return p.stringLiteral(), true
	}

	if p.peekType(INT_LITERAL) {
		if expectedType != nil && *expectedType != RslIntT {
			p.error("Expected int literal")
		}
		return p.intLiteral(), true
	}

	if p.peekType(FLOAT_LITERAL) {
		if expectedType != nil && *expectedType != RslFloatT {
			p.error("Expected float literal")
		}
		return p.floatLiteral(), true
	}

	// todo need to emit bool literal tokens
	if p.peekType(BOOL_LITERAL) {
		if expectedType != nil && *expectedType != RslBoolT {
			p.error("Expected bool literal")
		}
		return p.boolLiteral(), true
	}

	return nil, false
}

func (p *Parser) stringLiteral() StringLiteral {
	literal := p.consume(STRING_LITERAL, "Expected string literal").(*StringLiteralToken)
	return StringLiteral{Value: *literal}
}

func (p *Parser) intLiteral() IntLiteral {
	literal := p.consume(INT_LITERAL, "Expected int literal").(*IntLiteralToken)
	return IntLiteral{Value: *literal}
}

func (p *Parser) floatLiteral() FloatLiteral {
	literal := p.consume(FLOAT_LITERAL, "Expected float literal").(*FloatLiteralToken)
	return FloatLiteral{Value: *literal}
}

func (p *Parser) boolLiteral() BoolLiteral {
	literal := p.consume(BOOL_LITERAL, "Expected bool literal").(*BoolLiteralToken)
	return BoolLiteral{Value: *literal}
}

func (p *Parser) arrayLiteral(expectedType *RslTypeEnum) (ArrayLiteral, bool) {
	if p.matchAny(BRACKETS) {
		return &EmptyArrayLiteral{}, true
	}

	if !p.matchAny(LEFT_BRACKET) {
		return nil, false
	}
	p.rewind() // rewind the left bracket

	if expectedType == nil || *expectedType == RslArrayT {
		return p.mixedArrayLiteral(), true
	}

	switch *expectedType {
	case RslStringArrayT:
		return p.stringArrayLiteral(), true
	case RslIntArrayT:
		return p.intArrayLiteral(), true
	case RslFloatArrayT:
		return p.floatArrayLiteral(), true
	case RslBoolArrayT:
		return p.boolArrayLiteral(), true
	default:
		p.error("Invalid array type")
		panic(UNREACHABLE)
	}
}

func (p *Parser) stringArrayLiteral() StringArrayLiteral {
	if p.matchAny(BRACKETS) {
		return StringArrayLiteral{Values: []StringLiteral{}}
	}

	p.consume(LEFT_BRACKET, "Expected '[' to start array")

	var values []StringLiteral
	expectedType := RslStringT
	for !p.matchAny(RIGHT_BRACKET) {
		literal, ok := p.literal(&expectedType)
		if !ok {
			p.error("Expected string literal in array")
		}
		values = append(values, literal.(StringLiteral))
		if !p.peekType(RIGHT_BRACKET) {
			p.consume(COMMA, "Expected ',' between array elements")
		}
	}
	return StringArrayLiteral{Values: values}
}

func (p *Parser) intArrayLiteral() IntArrayLiteral {
	if p.matchAny(BRACKETS) {
		return IntArrayLiteral{Values: []IntLiteral{}}
	}

	p.consume(LEFT_BRACKET, "Expected '[' to start array")

	var values []IntLiteral
	expectedType := RslIntT
	for !p.matchAny(RIGHT_BRACKET) {
		literal, ok := p.literal(&expectedType)
		if !ok {
			p.error("Expected int literal in array")
		}
		values = append(values, literal.(IntLiteral))
		if !p.peekType(RIGHT_BRACKET) {
			p.consume(COMMA, "Expected ',' between array elements")
		}
	}
	return IntArrayLiteral{Values: values}
}

func (p *Parser) floatArrayLiteral() FloatArrayLiteral {
	if p.matchAny(BRACKETS) {
		return FloatArrayLiteral{Values: []FloatLiteral{}}
	}

	p.consume(LEFT_BRACKET, "Expected '[' to start array")

	var values []FloatLiteral
	expectedType := RslFloatT
	for !p.matchAny(RIGHT_BRACKET) {
		literal, ok := p.literal(&expectedType)
		if !ok {
			p.error("Expected float literal in array")
		}
		values = append(values, literal.(FloatLiteral))
		if !p.peekType(RIGHT_BRACKET) {
			p.consume(COMMA, "Expected ',' between array elements")
		}
	}
	return FloatArrayLiteral{Values: values}
}

func (p *Parser) boolArrayLiteral() BoolArrayLiteral {
	if p.matchAny(BRACKETS) {
		return BoolArrayLiteral{Values: []BoolLiteral{}}
	}

	p.consume(LEFT_BRACKET, "Expected '[' to start array")

	var values []BoolLiteral
	expectedType := RslBoolT
	for !p.matchAny(RIGHT_BRACKET) {
		literal, ok := p.literal(&expectedType)
		if !ok {
			p.error("Expected bool literal in array")
		}
		values = append(values, literal.(BoolLiteral))
		if !p.peekType(RIGHT_BRACKET) {
			p.consume(COMMA, "Expected ',' between array elements")
		}
	}
	return BoolArrayLiteral{Values: values}
}

func (p *Parser) mixedArrayLiteral() MixedArrayLiteral {
	if p.matchAny(BRACKETS) {
		return MixedArrayLiteral{Values: []LiteralOrArray{}}
	}

	p.consume(LEFT_BRACKET, "Expected '[' to start array")

	var values []LiteralOrArray
	for !p.matchAny(RIGHT_BRACKET) {
		literal, ok := p.literalOrArray(nil)
		if !ok {
			p.error("Expected literalOrArray in mixed array")
		}
		values = append(values, literal)
		if !p.peekType(RIGHT_BRACKET) {
			p.consume(COMMA, "Expected ',' between array elements")
		}
	}
	return MixedArrayLiteral{Values: values}
}

func (p *Parser) rslType() RslType {
	var argType Token
	var rslTypeEnum RslTypeEnum
	if p.matchKeyword(ARRAY, ARGS_BLOCK_KEYWORDS) { // todo technically this is used for typing in non-arg contexts too
		argType = p.previous()
		if p.matchAny(BRACKETS) {
			p.error("Brackets cannot follow 'array' type, just use 'array'.")
		} else {
			rslTypeEnum = RslArrayT
		}
	} else if p.matchKeyword(STRING, ARGS_BLOCK_KEYWORDS) {
		argType = p.previous()
		if p.matchAny(BRACKETS) {
			rslTypeEnum = RslStringArrayT
		} else {
			rslTypeEnum = RslStringT
		}
	} else if p.matchKeyword(INT, ARGS_BLOCK_KEYWORDS) {
		argType = p.previous()
		if p.matchAny(BRACKETS) {
			rslTypeEnum = RslIntArrayT
		} else {
			rslTypeEnum = RslIntT
		}
	} else if p.matchKeyword(FLOAT, ARGS_BLOCK_KEYWORDS) {
		argType = p.previous()
		if p.matchAny(BRACKETS) {
			rslTypeEnum = RslFloatArrayT
		} else {
			rslTypeEnum = RslFloatT
		}
	} else if p.matchKeyword(BOOL, ARGS_BLOCK_KEYWORDS) {
		argType = p.previous()
		if p.matchAny(BRACKETS) {
			rslTypeEnum = RslBoolArrayT
		} else {
			rslTypeEnum = RslBoolT
		}
	} else {
		p.error("Expected arg type")
	}
	return RslType{Token: argType, Type: rslTypeEnum}
}

func (p *Parser) identifier() Token {
	if p.matchAny(IDENTIFIER) {
		return p.previous()
	}
	p.error("Expected Identifier")
	panic(UNREACHABLE)
}

// todo putting this everywhere isn't ideal... another way to handle insignificant newlines?
func (p *Parser) consumeNewlines() {
	p.consumeNewlinesMin(0)
}

func (p *Parser) consumeNewlinesMin(min int) {
	matched := 0
	for !p.isAtEnd() && p.matchAny(NEWLINE) {
		// throw away
		matched++
	}
	if matched < min && !p.isAtEnd() {
		p.error("Expected newline")
	}
}

func (p *Parser) matchKeyword(tokenType TokenType, keywords map[string]TokenType) bool {
	next := p.peek()
	if next.GetType() != IDENTIFIER {
		return false
	}
	if keyword, ok := keywords[next.GetLexeme()]; ok {
		if keyword == tokenType {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) consumeKeyword(tokenType TokenType, keywords map[string]TokenType) Token {
	if !p.matchKeyword(tokenType, keywords) {
		p.error(fmt.Sprintf("Expected keyword %s", tokenType))
		panic(UNREACHABLE)
	}
	return p.previous()
}

func (p *Parser) peekKeyword(expectedKeyword TokenType, keywords map[string]TokenType) bool {
	if p.matchKeyword(expectedKeyword, keywords) {
		p.rewind()
		return true
	}
	return false
}

func (p *Parser) nextNonNewLineTokenIs(expected TokenType) bool {
	for !p.isAtEnd() && p.matchAny(NEWLINE) {
		// throw away
	}
	return p.peekType(expected)
}
