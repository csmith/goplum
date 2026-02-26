package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type token struct {
	Class  tokenClass
	Line   int
	Column int
	Value  any
}

type tokenClass int

const (
	tokenError         tokenClass = iota // Parsing error - value is the error text
	tokenEOF                             // End of file
	tokenBlockStart                      // {
	tokenBlockEnd                        // }
	tokenIdentifier                      // 'foo' in 'foo = "bar"', or 'foo { ... }'
	tokenAssignment                      // '=' in 'foo = bar'
	tokenString                          // "string" - value is unquoted string
	tokenInt                             // 123 - value is an int
	tokenFloat                           // 1.234 - value is a float32
	tokenDuration                        // 10d3m - value is a time.Duration
	tokenBoolean                         // true/false - value is a boolean
	tokenDelimiter                       // ,
	tokenFunctionStart                   // (
	tokenFunctionEnd                     // )
	tokenArrayStart                      // [
	tokenArrayEnd                        // ]
	tokenAlert                           // alert
	tokenDefaults                        // defaults
	tokenCheck                           // check
	tokenPlugin                          // plugin
	tokenGroup                           // group
)

var tokenNames = map[tokenClass]string{
	tokenError:         "error",
	tokenEOF:           "EOF",
	tokenBlockStart:    "start of block",
	tokenBlockEnd:      "end of block",
	tokenIdentifier:    "identifier",
	tokenAssignment:    "assignment",
	tokenString:        "quoted string",
	tokenInt:           "integer",
	tokenFloat:         "float",
	tokenDuration:      "duration",
	tokenBoolean:       "boolean",
	tokenDelimiter:     "delimiter",
	tokenFunctionStart: "start of function arguments",
	tokenFunctionEnd:   "end of function arguments",
	tokenArrayStart:    "start of array",
	tokenArrayEnd:      "end of array",
	tokenAlert:         "alert keyword",
	tokenDefaults:      "defaults keyword",
	tokenCheck:         "check keyword",
	tokenPlugin:        "plugin keyword",
	tokenGroup:         "group keyword",
}

var keywords = map[string]tokenClass{
	"defaults": tokenDefaults,
	"alert":    tokenAlert,
	"check":    tokenCheck,
	"plugin":   tokenPlugin,
	"group":    tokenGroup,
}

var booleans = map[string]bool{
	"true":  true,
	"on":    true,
	"yes":   true,
	"false": false,
	"off":   false,
	"no":    false,
}

var symbols = map[rune]tokenClass{
	'{': tokenBlockStart,
	'}': tokenBlockEnd,
	'=': tokenAssignment,
	',': tokenDelimiter,
	'(': tokenFunctionStart,
	')': tokenFunctionEnd,
	'[': tokenArrayStart,
	']': tokenArrayEnd,
}

var durations = map[rune]int{
	's': 1,
	'm': 60,
	'h': 60 * 60,
	'd': 60 * 60 * 24,
	'w': 60 * 60 * 24 * 7,
}

var stringEscapes = map[rune]rune{
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'\\': '\\',
	'"':  '"',
}

const eof = rune(-1)

type stateFunc func(*Lexer) stateFunc

type Lexer struct {
	line   int
	column int
	reader *bufio.Reader
	state  stateFunc
	output chan token
}

func NewLexer(reader io.Reader) *Lexer {
	return &Lexer{
		line:   1,
		column: 0,
		reader: bufio.NewReader(reader),
		state:  lex,
		output: make(chan token),
	}
}

func (l *Lexer) next() rune {
	r, _, err := l.reader.ReadRune()
	if err != nil {
		if err == io.EOF {
			return eof
		}
		panic(err)
	}
	l.column++
	return r
}

func (l *Lexer) backup() {
	if err := l.reader.UnreadRune(); err != nil {
		panic(err)
	}
	l.column--
}

func (l *Lexer) emit(class tokenClass, width int, value any) {
	l.output <- token{
		Class:  class,
		Line:   l.line,
		Column: l.column - width,
		Value:  value,
	}

	if class == tokenEOF || class == tokenError {
		close(l.output)
	}
}

func (l *Lexer) Lex() {
	for l.state != nil {
		l.state = l.state(l)
	}
}

func lex(lexer *Lexer) stateFunc {
	for {
		r := lexer.next()
		if r == eof {
			lexer.emit(tokenEOF, 0, nil)
			return nil
		} else if r == '\n' {
			lexer.line++
			lexer.column = 0
		} else if r == '"' {
			return lexString
		} else if r == '#' {
			return lexComment
		} else if unicode.IsLetter(r) {
			lexer.backup()
			return lexIdentifier
		} else if unicode.IsNumber(r) || r == '.' {
			lexer.backup()
			return lexNumber
		} else if t, ok := symbols[r]; ok {
			lexer.emit(t, 1, r)
		} else if unicode.IsSpace(r) {
			// Just carry on
		} else {
			lexer.emit(tokenError, 1, fmt.Sprintf("unexpected rune %c", r))
			return nil
		}
	}
}

func lexIdentifier(lexer *Lexer) stateFunc {
	builder := strings.Builder{}
	for {
		r := lexer.next()
		if r == eof {
			lexer.emit(tokenEOF, 0, nil)
			return nil
		} else if unicode.IsLetter(r) || r == '.' || r == '_' {
			builder.WriteRune(r)
		} else {
			lexer.backup()
			value := builder.String()
			name := strings.ToLower(value)

			if keyword, ok := keywords[name]; ok {
				lexer.emit(keyword, len(value), value)
			} else if boolean, ok := booleans[name]; ok {
				lexer.emit(tokenBoolean, len(value), boolean)
			} else {
				lexer.emit(tokenIdentifier, len(value), value)
			}
			return lex
		}
	}
}

func lexComment(lexer *Lexer) stateFunc {
	for {
		r := lexer.next()
		if r == eof {
			lexer.emit(tokenEOF, 0, nil)
			return nil
		} else if r == '\n' || r == '\r' {
			return lex
		}
	}
}

func lexString(lexer *Lexer) stateFunc {
	width := 1
	builder := strings.Builder{}
	escaped := false
	for {
		r := lexer.next()
		if r == eof || r == '\n' {
			lexer.emit(tokenError, width, "unterminated string literal")
			return nil
		} else if escaped {
			width++
			if sub, ok := stringEscapes[r]; ok {
				builder.WriteRune(sub)
				escaped = false
			} else {
				lexer.emit(tokenError, width, fmt.Sprintf("invalid character escape %c", r))
				return nil
			}
		} else if r == '\\' {
			escaped = true
			width++
		} else if r == '"' {
			width++
			lexer.emit(tokenString, width, builder.String())
			return lex
		} else {
			width++
			builder.WriteRune(r)
		}
	}
}

func lexNumber(lexer *Lexer) stateFunc {
	width := 0
	builder := strings.Builder{}
	isDuration := false
	hasPeriod := false
	duration := 0

	for {
		r := lexer.next()
		if unicode.IsNumber(r) {
			width++
			builder.WriteRune(r)
		} else if r == '.' {
			width++

			if hasPeriod {
				lexer.emit(tokenError, width, fmt.Sprintf("invalid numeric literal %s: multiple periods", builder.String()))
				return nil
			}

			builder.WriteRune(r)
			hasPeriod = true
		} else if m, ok := durations[r]; ok {
			width++

			value, err := strconv.Atoi(builder.String())
			if err != nil {
				lexer.emit(tokenError, width, fmt.Sprintf("invalid duration quantity '%s': %v", builder.String(), err))
				return nil
			}

			isDuration = true
			duration += m * value
			builder.Reset()
		} else if isDuration {
			lexer.backup()
			lexer.emit(tokenDuration, width, time.Duration(duration)*time.Second)
			return lex
		} else if hasPeriod {
			lexer.backup()
			value, err := strconv.ParseFloat(builder.String(), 64)
			if err != nil {
				lexer.emit(tokenError, width, fmt.Sprintf("invalid float literal '%s': %v", builder.String(), err))
				return nil
			}
			lexer.emit(tokenFloat, width, value)
			return lex
		} else {
			lexer.backup()
			value, err := strconv.Atoi(builder.String())
			if err != nil {
				lexer.emit(tokenError, width, fmt.Sprintf("invalid integer literal '%s': %v", builder.String(), err))
				return nil
			}
			lexer.emit(tokenInt, width, value)
			return lex
		}
	}
}
