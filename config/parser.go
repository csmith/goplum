package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dario.cat/mergo"
)

// Load parses the configuration file at path, along with any files it includes. Included files are resolved
// relative to the directory containing path.
func Load(path string) (*Config, error) {
	main, err := parseFile(path, false)
	if err != nil {
		return nil, err
	}

	res := main.Config
	for _, include := range main.Includes {
		files, err := resolveInclude(filepath.Dir(path), include.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", include.Position, err)
		}

		for _, file := range files {
			included, err := parseFile(file, true)
			if err != nil {
				return nil, err
			}

			res.AlertBlocks = append(res.AlertBlocks, included.AlertBlocks...)
			res.CheckBlocks = append(res.CheckBlocks, included.CheckBlocks...)
			res.PluginSettings = append(res.PluginSettings, included.PluginSettings...)
			res.GroupBlocks = append(res.GroupBlocks, included.GroupBlocks...)
		}
	}

	return &res, nil
}

func parseFile(path string, included bool) (*Parser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	parser := NewParser(f)
	parser.file = path
	parser.included = included
	if err := parser.Parse(); err != nil {
		return nil, fmt.Errorf("unable to parse config file %s: %v", path, err)
	}
	return parser, nil
}

func resolveInclude(dir, pattern string) ([]string, error) {
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(dir, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid include pattern %s: %v", pattern, err)
	}

	if len(matches) == 0 && !strings.ContainsAny(pattern, `*?[\`) {
		return nil, fmt.Errorf("included file %s does not exist", pattern)
	}

	slices.Sort(matches)
	return matches, nil
}

type Config struct {
	DefaultSettings map[string]any
	AlertBlocks     []*Block
	CheckBlocks     []*Block
	PluginSettings  []*Block
	GroupBlocks     []*Block
}

type Block struct {
	Name     string
	Type     string
	Settings map[string]any
	Position Position
}

// Position identifies where in the configuration a block was declared.
type Position struct {
	File string
	Line int
}

func (p Position) String() string {
	if p.File == "" {
		return fmt.Sprintf("line %d", p.Line)
	}
	return fmt.Sprintf("%s:%d", p.File, p.Line)
}

type Include struct {
	Pattern  string
	Position Position
}

type Parser struct {
	Config
	Includes    []Include
	lexer       *Lexer
	saved       *token
	last        *token
	file        string
	included    bool
	hasDefaults bool
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
		t, err := p.take(tokenEOF, tokenError, tokenDefaults, tokenInclude, tokenAlert, tokenCheck, tokenPlugin, tokenGroup)
		if err != nil {
			return err
		}

		switch t.Class {
		case tokenInclude:
			if err := p.parseInclude(t); err != nil {
				return err
			}
		case tokenDefaults:
			if err := p.parseDefaults(t); err != nil {
				return err
			}
		case tokenAlert:
			block, err := p.parseBlockWithTypeAndName(false)
			if err != nil {
				return err
			}
			block.Position = p.position(t)
			p.AlertBlocks = append(p.AlertBlocks, block)
		case tokenCheck:
			block, err := p.parseBlockWithTypeAndName(false)
			if err != nil {
				return err
			}
			block.Position = p.position(t)
			p.CheckBlocks = append(p.CheckBlocks, block)
		case tokenPlugin:
			block, err := p.parseBlockWithType(false)
			if err != nil {
				return err
			}
			block.Position = p.position(t)
			p.PluginSettings = append(p.PluginSettings, block)
		case tokenGroup:
			block, err := p.parseBlockWithName(true)
			if err != nil {
				return err
			}
			block.Position = p.position(t)
			p.GroupBlocks = append(p.GroupBlocks, block)
		case tokenError:
			return errors.New(t.Value.(string))
		case tokenEOF:
			return nil
		}
	}
}

func (p *Parser) parseInclude(t *token) error {
	if p.included {
		return fmt.Errorf("include not allowed in an included file at line %d", t.Line)
	}
	pattern, err := p.take(tokenString)
	if err != nil {
		return err
	}
	p.Includes = append(p.Includes, Include{Pattern: pattern.Value.(string), Position: p.position(t)})
	return nil
}

func (p *Parser) parseDefaults(t *token) error {
	if p.included {
		return fmt.Errorf("defaults block not allowed in an included file at line %d", t.Line)
	}
	if p.hasDefaults {
		return fmt.Errorf("duplicate defaults block declared at line %d", t.Line)
	}
	p.hasDefaults = true
	val, err := p.parseBlock(false)
	if err != nil {
		return err
	}
	return mergo.Merge(&p.DefaultSettings, val)
}

func (p *Parser) position(t *token) Position {
	return Position{File: p.file, Line: t.Line}
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
				if t.Class == tokenBoolean ||
					t.Class == tokenDuration ||
					t.Class == tokenFloat ||
					t.Class == tokenInt ||
					t.Class == tokenString {
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
