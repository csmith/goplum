package config

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"dario.cat/mergo"
)

type Block struct {
	Name     string
	Type     string
	Settings map[string]any
}

type Parser struct {
	lexer           *Lexer
	saved           *token
	last            *token
	hasDefaults     bool
	DefaultSettings map[string]any
	AlertBlocks     []*Block
	CheckBlocks     []*Block
	PluginSettings  []*Block
	GroupBlocks     []*Block
}

func NewParser(reader io.Reader) *Parser {
	return &Parser{
		lexer:           NewLexer(reader),
		DefaultSettings: make(map[string]any),
	}
}

func (p *Parser) Parse() error {
	go p.lexer.Lex()

	for {
		t, err := p.take(tokenEOF, tokenError, tokenDefaults, tokenAlert, tokenCheck, tokenPlugin, tokenGroup)
		if err != nil {
			return err
		}

		switch t.Class {
		case tokenDefaults:
			if p.hasDefaults {
				return fmt.Errorf("duplicate defaults block declared at line %d", t.Line)
			}
			p.hasDefaults = true
			val, err := p.parseBlock(false)
			if err != nil {
				return err
			}
			if err := mergo.Merge(&p.DefaultSettings, val); err != nil {
				return err
			}
		case tokenAlert:
			block, err := p.parseBlockWithTypeAndName(false)
			if err != nil {
				return err
			}
			p.AlertBlocks = append(p.AlertBlocks, block)
		case tokenCheck:
			block, err := p.parseBlockWithTypeAndName(false)
			if err != nil {
				return err
			}
			p.CheckBlocks = append(p.CheckBlocks, block)
		case tokenPlugin:
			block, err := p.parseBlockWithType(false)
			if err != nil {
				return err
			}
			p.PluginSettings = append(p.PluginSettings, block)
		case tokenGroup:
			block, err := p.parseBlockWithName(true)
			if err != nil {
				return err
			}
			p.GroupBlocks = append(p.GroupBlocks, block)
		case tokenError:
			return errors.New(t.Value.(string))
		case tokenEOF:
			return nil
		}
	}
}

func (p *Parser) parseBlockWithTypeAndName(allowDefaults bool) (*Block, error) {
	kind, err := p.take(tokenIdentifier)
	if err != nil {
		return nil, err
	}
	name, err := p.take(tokenString)
	if err != nil {
		return nil, err
	}
	val, err := p.parseBlock(allowDefaults)
	if err != nil {
		return nil, err
	}
	return &Block{
		Name:     name.Value.(string),
		Type:     kind.Value.(string),
		Settings: val,
	}, nil
}

func (p *Parser) parseBlockWithType(allowDefaults bool) (*Block, error) {
	kind, err := p.take(tokenIdentifier)
	if err != nil {
		return nil, err
	}
	val, err := p.parseBlock(allowDefaults)
	if err != nil {
		return nil, err
	}
	return &Block{
		Type:     kind.Value.(string),
		Settings: val,
	}, nil
}

func (p *Parser) parseBlockWithName(allowDefaults bool) (*Block, error) {
	name, err := p.take(tokenString)
	if err != nil {
		return nil, err
	}
	val, err := p.parseBlock(allowDefaults)
	if err != nil {
		return nil, err
	}
	return &Block{
		Name:     name.Value.(string),
		Settings: val,
	}, nil
}

func (p *Parser) parseBlock(allowDefaults bool) (map[string]any, error) {
	if _, err := p.take(tokenBlockStart); err != nil {
		return nil, err
	}

	res := make(map[string]any)
	for {
		var wanted = []tokenClass{tokenBlockEnd, tokenIdentifier}
		if allowDefaults {
			wanted = append(wanted, tokenDefaults)
		}

		t, err := p.take(wanted...)
		if err != nil {
			return nil, err
		}

		switch t.Class {
		case tokenBlockEnd:
			return res, nil
		case tokenDefaults:
			if _, ok := res["defaults"]; ok {
				return nil, fmt.Errorf("defaults block redeclared at line %d, column %d", t.Line, t.Column)
			}
			val, err := p.parseBlock(false)
			if err != nil {
				return nil, err
			}
			res["defaults"] = val
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

func (p *Parser) parseAssignment() (any, error) {
	s, err := p.take(tokenAssignment, tokenBlockStart)
	if err != nil {
		return nil, err
	}
	if s.Class == tokenBlockStart {
		p.backup()
		return p.parseBlock(false)
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

func (p *Parser) parseSequence(start, end tokenClass) ([]any, error) {
	if _, err := p.take(start); err != nil {
		return nil, err
	}

	var values []any
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

	if slices.Contains(classes, t.Class) {
		return t, nil
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
		"unexpected %s (%v) at line %d column %d, expecting one of: %s",
		tokenNames[token.Class],
		token.Value,
		token.Line,
		token.Column,
		strings.Join(names, ", "),
	)
}
