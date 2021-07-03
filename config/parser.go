package config

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/imdario/mergo"
)

type Block struct {
	Name     string
	Type     string
	Settings map[string]interface{}
}

type Parser struct {
	lexer           *Lexer
	saved           *token
	last            *token
	hasDefaults     bool
	DefaultSettings map[string]interface{}
	AlertBlocks     []*Block
	CheckBlocks     []*Block
	PluginSettings  []*Block
}

func NewParser(reader io.Reader) *Parser {
	return &Parser{
		lexer:           NewLexer(reader),
		DefaultSettings: make(map[string]interface{}),
	}
}

func (p *Parser) Parse() error {
	go p.lexer.Lex()

	for {
		t, err := p.take(tokenEOF, tokenError, tokenDefaults, tokenAlert, tokenCheck, tokenPlugin)
		if err != nil {
			return err
		}

		switch t.Class {
		case tokenDefaults:
			if p.hasDefaults {
				return fmt.Errorf("duplicate defaults block declared at line %d", t.Line)
			}
			p.hasDefaults = true
			val, err := p.parseBlock()
			if err != nil {
				return err
			}
			if err := mergo.Merge(&p.DefaultSettings, val); err != nil {
				return err
			}
		case tokenAlert:
			block, err := p.parseNamedBlock()
			if err != nil {
				return err
			}
			p.AlertBlocks = append(p.AlertBlocks, block)
		case tokenCheck:
			block, err := p.parseNamedBlock()
			if err != nil {
				return err
			}
			p.CheckBlocks = append(p.CheckBlocks, block)
		case tokenPlugin:
			block, err := p.parseTypedBlock()
			if err != nil {
				return err
			}
			p.PluginSettings = append(p.PluginSettings, block)
		case tokenError:
			return errors.New(t.Value.(string))
		case tokenEOF:
			return nil
		}
	}
}

func (p *Parser) parseNamedBlock() (*Block, error) {
	kind, err := p.take(tokenIdentifier)
	if err != nil {
		return nil, err
	}
	name, err := p.take(tokenString)
	if err != nil {
		return nil, err
	}
	val, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &Block{
		Name:     name.Value.(string),
		Type:     kind.Value.(string),
		Settings: val,
	}, nil
}

func (p *Parser) parseTypedBlock() (*Block, error) {
	kind, err := p.take(tokenIdentifier)
	if err != nil {
		return nil, err
	}
	val, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &Block{
		Type:     kind.Value.(string),
		Settings: val,
	}, nil
}

func (p *Parser) parseBlock() (map[string]interface{}, error) {
	if _, err := p.take(tokenBlockStart); err != nil {
		return nil, err
	}

	res := make(map[string]interface{})
	for {
		t, err := p.take(tokenBlockEnd, tokenIdentifier)
		if err != nil {
			return nil, err
		}

		switch t.Class {
		case tokenBlockEnd:
			return res, nil
		case tokenIdentifier:
			name := t.Value.(string)
			if _, ok := res[name]; ok {
				return nil, fmt.Errorf("attribute '%s' redeclared at line %d, column %d", name, t.Line, t.Column)
			}
			val, err := p.parseAssignment()
			if err != nil {
				return nil, err
			}
			res[name] = val
		}
	}
}

func (p *Parser) parseAssignment() (interface{}, error) {
	s, err := p.take(tokenAssignment, tokenBlockStart)
	if err != nil {
		return nil, err
	}
	if s.Class == tokenBlockStart {
		p.backup()
		return p.parseBlock()
	}

	n, err := p.take(tokenString, tokenDuration, tokenInt, tokenFloat, tokenBoolean, tokenArrayStart)
	if err != nil {
		return nil, err
	}

	if n.Class == tokenArrayStart {
		p.backup()
		return p.parseSequence(tokenArrayStart, tokenArrayEnd)
	} else {
		return n.Value, nil
	}
}

func (p *Parser) parseSequence(start, end tokenClass) ([]interface{}, error) {
	if _, err := p.take(start); err != nil {
		return nil, err
	}

	var values []interface{}
	var delim = false
	var firstType = tokenError

	for {
		var wanted []tokenClass
		if delim {
			wanted = []tokenClass{tokenDelimiter, end}
		} else {
			wanted = []tokenClass{tokenBoolean, tokenDuration, tokenFloat, tokenInt, tokenString, end}
		}

		t, err := p.take(wanted...)
		if err != nil {
			return nil, err
		}

		switch t.Class {
		case end:
			return values, nil
		case tokenDelimiter:
			break
		case firstType:
			values = append(values, t.Value)
		default:
			if firstType == tokenError {
				if t.Class == tokenBoolean || t.Class == tokenDuration || t.Class == tokenFloat || t.Class == tokenInt || t.Class == tokenString {
					firstType = t.Class
					values = append(values, t.Value)
				} else {
					return nil, unexpected(t, tokenBoolean, tokenDuration, tokenFloat, tokenInt, tokenString, end)
				}
			} else {
				return nil, unexpected(t, firstType, end)
			}
		}

		delim = !delim
	}
}

func (p *Parser) next() *token {
	if p.saved == nil {
		token := <-p.lexer.output
		p.last = &token
	} else {
		p.last = p.saved
	}

	p.saved = nil
	return p.last
}

func (p *Parser) backup() {
	p.saved = p.last
}

func (p *Parser) take(classes ...tokenClass) (*token, error) {
	t := p.next()

	for i := range classes {
		if t.Class == classes[i] {
			return t, nil
		}
	}

	if t.Class == tokenEOF {
		return nil, io.EOF
	}

	return nil, unexpected(t, classes...)
}

func unexpected(token *token, expected ...tokenClass) error {
	names := make([]string, len(expected))
	for i := range expected {
		names[i] = tokenNames[expected[i]]
	}

	return fmt.Errorf(
		"unexpected %s (%s) at line %d column %d, expecting one of: %s",
		tokenNames[token.Class],
		token.Value,
		token.Line,
		token.Column,
		strings.Join(names, ", "),
	)
}
