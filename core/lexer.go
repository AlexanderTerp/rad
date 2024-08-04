package core

import (
	"fmt"
	"strconv"
	"strings"
)

type Lexer struct {
	source        string
	start         int // index of start of the current lexeme (0 indexed)
	next          int // index of next character to be read (0 indexed)
	lineIndex     int // current line number (1 indexed)
	nextLineIndex int // character number in the current line (1 indexed)
	Tokens        []Token
}

func NewLexer(source string) *Lexer {
	return &Lexer{
		source:        source,
		start:         0,
		next:          0,
		lineIndex:     1,
		nextLineIndex: 0,
		Tokens:        []Token{},
	}
}

func (l *Lexer) Lex() []Token {
	for !l.isAtEnd() {
		l.scanToken()
	}

	l.Tokens = append(l.Tokens, NewToken(EOF, "", l.next, l.lineIndex, l.nextLineIndex))
	return l.Tokens
}

func (l *Lexer) isAtEnd() bool {
	return l.next >= len(l.source)
}

func (l *Lexer) scanToken() {
	l.start = l.next
	c := l.advance()
	switch c {
	case '(':
		l.addToken(LEFT_PAREN)
	case ')':
		l.addToken(RIGHT_PAREN)
	case ',':
		l.addToken(COMMA)
	case ':':
		l.addToken(COLON)
	case '\n':
		l.addToken(NEWLINE)
	case '=':
		if l.match('=') {
			l.addToken(EQUAL_EQUAL)
		} else {
			l.addToken(EQUAL)
		}
	case '!':
		if l.match('=') {
			l.addToken(NOT_EQUAL)
		} else {
			l.addToken(EXCLAMATION)
		}
	case '<':
		if l.match('=') {
			l.addToken(LESS_EQUAL)
		} else {
			l.addToken(LESS)
		}
	case '>':
		if l.match('=') {
			l.addToken(GREATER_EQUAL)
		} else {
			l.addToken(GREATER)
		}
	case '|':
		l.addToken(PIPE)
	case '+':
		l.addToken(PLUS)
	case '-':
		l.addToken(MINUS)
	case '@':
		l.addToken(AT)
	case '#':
		l.lexArgComment()
	case '"':
		if l.match('"') {
			if l.match('"') {
				if !l.match('\n') {
					l.error("Expected newline after triple quote")
				} else {
					l.lineIndex++
					l.nextLineIndex = 0
					l.lexFileHeader()
				}
			} else {
				literal := ""
				l.addStringLiteralToken(&literal)
			}
		} else {
			l.lexStringLiteral()
		}
	case 'j':
		if l.matchString("son") {
			l.lexJsonPath()
		}
	case '/':
		if l.match('/') {
			for l.peek() != '\n' && !l.isAtEnd() {
				l.advance()
			}
		} else {
			l.error("Unexpected /")
		}
	case ' ', '\t', '\r':
		// todo handle indentations!
	default:
		if isDigit(c) {
			l.lexInt()
		} else if isAlpha(c) {
			l.lexIdentifier()
		} else {
			l.error("Unexpected character")
		}
	}

}

func (l *Lexer) advance() rune {
	r := rune(l.source[l.next])
	if r == '\n' {
		l.lineIndex++
		l.nextLineIndex = 0
	} else {
		l.nextLineIndex++
	}
	l.next++
	return r
}

func (l *Lexer) match(expected rune) bool {
	return l.matchAny(expected)
}

func (l *Lexer) matchAny(expected ...rune) bool {
	if l.isAtEnd() {
		return false
	}

	nextRune := rune(l.source[l.next])
	for _, r := range expected {
		if nextRune == r {
			if nextRune == '\n' {
				l.lineIndex++
				l.nextLineIndex = 0
			} else {
				l.nextLineIndex++
			}
			l.next++
			return true
		}
	}
	return false
}

func (l *Lexer) matchString(expected string) bool {
	for i, c := range expected {
		if l.next+i >= len(l.source) || rune(l.source[l.next+i]) != c {
			return false
		}
	}
	l.next += len(expected)
	return true
}

func (l *Lexer) peekMatch(toCheck string) bool {
	for i, c := range toCheck {
		if l.next+i >= len(l.source) || rune(l.source[l.next+i]) != c {
			return false
		}
	}
	return true
}

func (l *Lexer) peek() rune {
	if l.isAtEnd() {
		return 0
	}
	return rune(l.source[l.next])
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func (l *Lexer) lexStringLiteral() {
	value := ""
	for !l.match('"') && !l.isAtEnd() {
		value = value + string(l.advance())
	}
	l.addStringLiteralToken(&value)
}

func (l *Lexer) lexInt() {
	for isDigit(l.peek()) {
		l.advance()
	}
	lexeme := l.source[l.start:l.next]
	literal, err := strconv.Atoi(lexeme)
	if err != nil {
		l.error("Invalid integer")
	}
	l.addIntLiteralToken(literal)
}

func (l *Lexer) lexIdentifier() {
	for isAlpha(l.peek()) {
		l.advance()
	}
	l.addToken(IDENTIFIER)
}

func (l *Lexer) lexJsonPath() {
	panic("not implemented")
}

func (l *Lexer) lexArgComment() {
	for l.peek() != '\n' && !l.isAtEnd() {
		l.advance()
	}

	value := strings.TrimSpace(l.source[l.start+1 : l.next])
	l.addArgCommentLiteralToken(&value)
}

func (l *Lexer) lexFileHeader() {
	value := ""
	for !l.matchString("\n\"\"\"") {
		value = value + string(l.advance())
	}
	l.advance()
	l.advance()
	l.addStringLiteralToken(&value) // todo should this be its own literal type?
}

func (l *Lexer) addToken(tokenType TokenType) {
	lexeme := l.source[l.start:l.next]
	token := NewToken(tokenType, lexeme, l.start, l.lineIndex, l.nextLineIndex)
	l.Tokens = append(l.Tokens, token)
}

func (l *Lexer) addStringLiteralToken(literal *string) {
	lexeme := l.source[l.start:l.next]
	token := NewStringLiteralToken(STRING_LITERAL, lexeme, l.start, l.lineIndex, l.nextLineIndex, literal)
	l.Tokens = append(l.Tokens, token)
}

func (l *Lexer) addIntLiteralToken(literal int) {
	lexeme := l.source[l.start:l.next]
	token := NewIntLiteralToken(INT_LITERAL, lexeme, l.start, l.lineIndex, l.nextLineIndex, &literal)
	l.Tokens = append(l.Tokens, token)
}

func (l *Lexer) addArgCommentLiteralToken(comment *string) {
	lexeme := l.source[l.start:l.next]
	token := NewArgCommentLiteralToken(ARG_COMMENT, lexeme, l.start, l.lineIndex, l.nextLineIndex, comment)
	l.Tokens = append(l.Tokens, token)
}

func (l *Lexer) error(message string) {
	lexeme := l.source[l.start:l.next]
	panic(fmt.Sprintf("Error at L%d/%d on '%s': %s", l.lineIndex, l.nextLineIndex, lexeme, message))
}
