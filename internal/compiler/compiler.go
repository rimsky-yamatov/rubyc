// Package compiler contains the Ruby subset frontend and TinyCC driver.
package compiler

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Compile translates the supported Ruby subset to a standalone C program.
func Compile(source, filename string) (string, error) {
	p := newParser(source, filename)
	stmts, err := p.program("")
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	b.WriteString(runtimeC)
	b.WriteString("\nint main(void){\n")
	for _, s := range stmts {
		b.WriteString(s.c(1))
	}
	b.WriteString("return 0;\n}\n")
	return b.String(), nil
}

type expr interface{ c() string }
type stmt interface{ c(int) string }
type literal struct{ text string }

func (e literal) c() string { return e.text }

type binary struct {
	op   string
	a, b expr
}

func (e binary) c() string {
	if e.op == "+" {
		return "add(" + e.a.c() + "," + e.b.c() + ")"
	}
	if e.op == "==" {
		return "num(eq(" + e.a.c() + "," + e.b.c() + "))"
	}
	if e.op == "!=" {
		return "num(!eq(" + e.a.c() + "," + e.b.c() + "))"
	}
	return "num((" + e.a.c() + ").n" + e.op + "(" + e.b.c() + ").n)"
}

type unary struct {
	op    string
	value expr
}

func (e unary) c() string {
	if e.op == "!" {
		return "num(!truth(" + e.value.c() + "))"
	}
	return "num(-(" + e.value.c() + ").n)"
}

// call lowers selected String and Kernel methods to the embedded runtime.
type call struct {
	receiver expr
	name     string
}

func (e call) c() string {
	if e.receiver == nil && e.name == "gets" {
		return "input()"
	}
	if e.name == "to_s" {
		return "str(text(" + e.receiver.c() + "))"
	}
	methods := map[string]string{"length": "length", "size": "length", "upcase": "upper", "downcase": "lower", "chomp": "chomp"}
	return methods[e.name] + "(" + e.receiver.c() + ")"
}

type assign struct {
	name  string
	value expr
	decl  bool
}

func (s assign) c(i int) string {
	prefix := ""
	if s.decl {
		prefix = "Value "
	}
	return strings.Repeat("\t", i) + prefix + s.name + "=" + s.value.c() + ";\n"
}

type output struct {
	value expr
	nl    bool
}

func (s output) c(i int) string {
	nl := "0"
	if s.nl {
		nl = "1"
	}
	return strings.Repeat("\t", i) + "out(" + s.value.c() + "," + nl + ");\n"
}

type conditional struct {
	test    expr
	yes, no []stmt
}

func (s conditional) c(i int) string {
	pad := strings.Repeat("\t", i)
	var b strings.Builder
	b.WriteString(pad + "if (truth(" + s.test.c() + ")) {\n")
	for _, x := range s.yes {
		b.WriteString(x.c(i + 1))
	}
	b.WriteString(pad + "}")
	if len(s.no) > 0 {
		b.WriteString(" else {\n")
		for _, x := range s.no {
			b.WriteString(x.c(i + 1))
		}
		b.WriteString(pad + "}")
	}
	return b.String() + "\n"
}

type loop struct {
	test expr
	body []stmt
}

func (s loop) c(i int) string {
	pad := strings.Repeat("\t", i)
	var b strings.Builder
	b.WriteString(pad + "while (truth(" + s.test.c() + ")) {\n")
	for _, x := range s.body {
		b.WriteString(x.c(i + 1))
	}
	return b.String() + pad + "}\n"
}

type parser struct {
	lines []string
	file  string
	line  int
	vars  map[string]bool
}

func newParser(s, file string) *parser {
	return &parser{lines: strings.Split(s, "\n"), file: file, vars: map[string]bool{}}
}
func (p *parser) problem(format string, a ...any) error {
	return fmt.Errorf("%s:%d: %s", p.file, p.line+1, fmt.Sprintf(format, a...))
}
func (p *parser) program(stop string) ([]stmt, error) {
	var result []stmt
	for p.line < len(p.lines) {
		raw := strings.TrimSpace(strings.SplitN(p.lines[p.line], "#", 2)[0])
		p.line++
		if raw == "" {
			continue
		}
		if raw == "end" || raw == "else" {
			if stop != "" {
				return result, nil
			}
			return nil, p.problem("unexpected %q", raw)
		}
		if strings.HasPrefix(raw, "if ") || strings.HasPrefix(raw, "unless ") {
			unless := strings.HasPrefix(raw, "unless ")
			prefix := "if "
			if unless {
				prefix = "unless "
			}
			test, err := p.expression(strings.TrimSpace(raw[len(prefix):]))
			if err != nil {
				return nil, err
			}
			if unless {
				test = unary{"!", test}
			}
			yes, err := p.program("end")
			if err != nil {
				return nil, err
			}
			var no []stmt
			if p.line <= len(p.lines) && strings.TrimSpace(strings.SplitN(p.lines[p.line-1], "#", 2)[0]) == "else" {
				no, err = p.program("end")
				if err != nil {
					return nil, err
				}
			}
			if p.line > len(p.lines) || strings.TrimSpace(strings.SplitN(p.lines[p.line-1], "#", 2)[0]) != "end" {
				return nil, p.problem("if without matching end")
			}
			result = append(result, conditional{test, yes, no})
			continue
		}
		if strings.HasPrefix(raw, "while ") {
			test, err := p.expression(strings.TrimSpace(raw[len("while "):]))
			if err != nil {
				return nil, err
			}
			body, err := p.program("end")
			if err != nil {
				return nil, err
			}
			if p.line > len(p.lines) || strings.TrimSpace(p.lines[p.line-1]) != "end" {
				return nil, p.problem("while without matching end")
			}
			result = append(result, loop{test, body})
			continue
		}
		if strings.HasPrefix(raw, "puts ") || strings.HasPrefix(raw, "print ") {
			nl := strings.HasPrefix(raw, "puts ")
			keywordLen := len("print ")
			if nl {
				keywordLen = len("puts ")
			}
			e, err := p.expression(strings.TrimSpace(raw[keywordLen:]))
			if err != nil {
				return nil, err
			}
			result = append(result, output{e, nl})
			continue
		}
		if n, v, ok := strings.Cut(raw, "="); ok && !strings.Contains(v, "=") && isIdent(strings.TrimSpace(n)) {
			e, err := p.expression(strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
			name := strings.TrimSpace(n)
			decl := !p.vars[name]
			p.vars[name] = true
			result = append(result, assign{name, e, decl})
			continue
		}
		return nil, p.problem("unsupported syntax: %s", raw)
	}
	if stop != "" {
		return nil, p.problem("missing end")
	}
	return result, nil
}
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
func (p *parser) expression(s string) (expr, error) {
	parts, err := lexExpression(s)
	if err != nil {
		return nil, p.problem("%v", err)
	}
	if len(parts) == 0 {
		return nil, p.problem("expected expression")
	}
	pos := 0
	var parse func(int) (expr, error)
	atom := func(x string) (expr, error) {
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return literal{"num(" + strconv.FormatInt(n, 10) + ")"}, nil
		}
		if len(x) >= 2 && x[0] == '"' && x[len(x)-1] == '"' {
			q, err := strconv.Unquote(x)
			if err != nil {
				return nil, p.problem("invalid string")
			}
			return literal{"str(" + strconv.Quote(q) + ")"}, nil
		}
		if x == "true" {
			return literal{"num(1)"}, nil
		}
		if x == "false" {
			return literal{"num(0)"}, nil
		}
		if x == "nil" {
			return literal{"nil()"}, nil
		}
		if p.vars[x] {
			return literal{x}, nil
		}
		return nil, p.problem("unknown value %q", x)
	}
	precedence := map[string]int{"==": 1, "!=": 1, "<": 2, "<=": 2, ">": 2, ">=": 2, "+": 3, "-": 3, "*": 4, "/": 4}
	parse = func(min int) (expr, error) {
		if pos >= len(parts) {
			return nil, p.problem("expected expression")
		}
		tok := parts[pos]
		pos++
		var left expr
		var e error
		if tok == "(" {
			left, e = parse(0)
			if e != nil {
				return nil, e
			}
			if pos >= len(parts) || parts[pos] != ")" {
				return nil, p.problem("missing )")
			}
			pos++
		} else if tok == "!" || tok == "-" {
			v, err := parse(5)
			if err != nil {
				return nil, err
			}
			left = unary{tok, v}
		} else if tok == "gets" {
			left = call{name: "gets"}
		} else {
			left, e = atom(tok)
			if e != nil {
				return nil, e
			}
		}
		for pos < len(parts) {
			if parts[pos] == "." {
				pos++
				if pos >= len(parts) || !isIdent(parts[pos]) {
					return nil, p.problem("expected method name")
				}
				name := parts[pos]
				pos++
				if name != "length" && name != "size" && name != "upcase" && name != "downcase" && name != "chomp" && name != "to_s" {
					return nil, p.problem("unsupported method %q", name)
				}
				left = call{receiver: left, name: name}
				continue
			}
			op := parts[pos]
			prec, ok := precedence[op]
			if !ok || prec < min {
				break
			}
			pos++
			right, err := parse(prec + 1)
			if err != nil {
				return nil, err
			}
			left = binary{op, left, right}
		}
		return left, nil
	}
	e, err := parse(0)
	if err != nil {
		return nil, err
	}
	if pos != len(parts) {
		return nil, p.problem("unexpected token %q", parts[pos])
	}
	return e, nil
}
func lexExpression(s string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		if strings.ContainsRune("+-*/()!<>==.", rune(s[i])) {
			if i+1 < len(s) && strings.ContainsRune("=!<>", rune(s[i])) && s[i+1] == '=' {
				tokens = append(tokens, s[i:i+2])
				i += 2
				continue
			}
			tokens = append(tokens, s[i:i+1])
			i++
			continue
		}
		if s[i] == '"' {
			start := i
			i++
			closed := false
			escaped := false
			for i < len(s) {
				if s[i] == '"' && !escaped {
					i++
					tokens = append(tokens, s[start:i])
					closed = true
					break
				}
				escaped = s[i] == '\\' && !escaped
				if s[i] != '\\' {
					escaped = false
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated string")
			}
			continue
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && !strings.ContainsRune("+-*/()!<>==.", rune(s[i])) {
			i++
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens, nil
}
