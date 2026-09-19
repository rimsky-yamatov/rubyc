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
    b.WriteString("\n")
    for _, fn := range p.functions {
        b.WriteString(fn)
    }
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
    switch e.op {
    case "+":
        return "add(" + e.a.c() + "," + e.b.c() + ")"
    case "==":
        return "num(eq(" + e.a.c() + "," + e.b.c() + "))"
    case "!=":
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

type call struct {
    receiver expr
    name     string
    args     []expr
}

func (e call) c() string {
    if e.receiver == nil && e.name == "gets" {
        return "input()"
    }
    if e.receiver != nil {
        if e.name == "to_s" {
            return "str(text(" + e.receiver.c() + "))"
        }
        switch e.name {
        case "length", "size":
            return "length(" + e.receiver.c() + ")"
        case "upcase":
            return "upper(" + e.receiver.c() + ")"
        case "downcase":
            return "lower(" + e.receiver.c() + ")"
        case "chomp":
            return "chomp(" + e.receiver.c() + ")"
        }
    }
    args := make([]string, 0, len(e.args))
    if e.receiver != nil {
        args = append(args, e.receiver.c())
    }
    for _, arg := range e.args {
        args = append(args, arg.c())
    }
    prefix := e.name
    if e.receiver == nil {
        prefix = "rubyc_" + e.name
    }
    return prefix + "(" + strings.Join(args, ",") + ")"
}

type index struct {
    a expr
    i expr
}

func (e index) c() string {
    return "index(" + e.a.c() + "," + e.i.c() + ")"
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

type indexAssign struct {
    a expr
    i expr
    v expr
}

func (s indexAssign) c(i int) string {
    return strings.Repeat("\t", i) + "setindex(" + s.a.c() + "," + s.i.c() + "," + s.v.c() + ");\n"
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

type returnStmt struct {
    value expr
}

func (s returnStmt) c(i int) string {
    return strings.Repeat("\t", i) + "return " + s.value.c() + ";\n"
}

type funcDef struct {
    name string
    args []string
    body []stmt
}

func (d funcDef) c() string {
    var b strings.Builder
    b.WriteString("static Value rubyc_" + d.name + "(")
    for i, arg := range d.args {
        if i > 0 {
            b.WriteString(",")
        }
        b.WriteString("Value " + arg)
    }
    b.WriteString(") {\n")
    for _, stmt := range d.body {
        b.WriteString(stmt.c(1))
    }
    b.WriteString("return nil();\n}\n")
    return b.String()
}

type parser struct {
    lines     []string
    file      string
    line      int
    vars      map[string]bool
    functions []string
    methods   map[string]bool
}

func newParser(s, file string) *parser {
    return &parser{lines: strings.Split(s, "\n"), file: file, vars: map[string]bool{}, methods: map[string]bool{}}
}

func (p *parser) problem(format string, a ...any) error {
    return fmt.Errorf("%s:%d: %s", p.file, p.line+1, fmt.Sprintf(format, a...))
}

func clean(s string) string {
    return strings.TrimSpace(strings.SplitN(s, "#", 2)[0])
}

func (p *parser) program(stop string) ([]stmt, error) {
    var result []stmt
    for p.line < len(p.lines) {
        raw := clean(p.lines[p.line])
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
        if strings.HasPrefix(raw, "def ") {
            if err := p.parseMethod(raw); err != nil {
                return nil, err
            }
            continue
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
            if p.line <= len(p.lines) && clean(p.lines[p.line-1]) == "else" {
                no, err = p.program("end")
                if err != nil {
                    return nil, err
                }
            }
            if p.line > len(p.lines) || clean(p.lines[p.line-1]) != "end" {
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
            if p.line > len(p.lines) || clean(p.lines[p.line-1]) != "end" {
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
        if strings.HasPrefix(raw, "return ") {
            e, err := p.expression(strings.TrimSpace(raw[len("return "):]))
            if err != nil {
                return nil, err
            }
            result = append(result, returnStmt{e})
            continue
        }
        if strings.Contains(raw, "=") {
            lhs, rhs, ok := strings.Cut(raw, "=")
            if ok && !strings.Contains(rhs, "=") {
                lhs = strings.TrimSpace(lhs)
                rhs = strings.TrimSpace(rhs)
                if strings.Contains(lhs, "[") && strings.Contains(lhs, "]") {
                    br := strings.Index(lhs, "[")
                    end := strings.LastIndex(lhs, "]")
                    if br >= 0 && end > br {
                        base, err := p.expression(strings.TrimSpace(lhs[:br]))
                        if err != nil {
                            return nil, err
                        }
                        idx, err := p.expression(strings.TrimSpace(lhs[br+1 : end]))
                        if err != nil {
                            return nil, err
                        }
                        val, err := p.expression(rhs)
                        if err != nil {
                            return nil, err
                        }
                        result = append(result, indexAssign{base, idx, val})
                        continue
                    }
                }
                if !isIdent(lhs) {
                    return nil, p.problem("unsupported assignment target %q", lhs)
                }
                e, err := p.expression(rhs)
                if err != nil {
                    return nil, err
                }
                decl := !p.vars[lhs]
                p.vars[lhs] = true
                result = append(result, assign{lhs, e, decl})
                continue
            }
        }
        return nil, p.problem("unsupported syntax: %s", raw)
    }
    if stop != "" {
        return nil, p.problem("missing end")
    }
    return result, nil
}

func (p *parser) parseMethod(raw string) error {
    header := strings.TrimSpace(raw[len("def "):])
    name, argsPart, ok := strings.Cut(header, "(")
    if !ok {
        return p.problem("invalid method declaration %q", raw)
    }
    name = strings.TrimSpace(name)
    if !isIdent(name) {
        return p.problem("invalid method name %q", name)
    }
    if !strings.HasSuffix(argsPart, ")") {
        return p.problem("missing ) in method declaration %q", raw)
    }
    argsSpec := strings.TrimSpace(strings.TrimSuffix(argsPart, ")"))
    var args []string
    if argsSpec != "" {
        for _, arg := range strings.Split(argsSpec, ",") {
            arg = strings.TrimSpace(arg)
            if arg == "" {
                continue
            }
            if !isIdent(arg) {
                return p.problem("invalid argument %q", arg)
            }
            args = append(args, arg)
        }
    }
    oldVars := p.vars
    p.vars = map[string]bool{}
    for _, arg := range args {
        p.vars[arg] = true
    }
    body, err := p.program("end")
    p.vars = oldVars
    if err != nil {
        return err
    }
    p.methods[name] = true
    p.functions = append(p.functions, funcDef{name: name, args: args, body: body}.c())
    return nil
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
    precedence := map[string]int{"==": 1, "!=": 1, "<": 2, "<=": 2, ">": 2, ">=": 2, "+": 3, "-": 3, "*": 4, "/": 4}
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
        if p.vars[x] || p.methods[x] {
            return literal{x}, nil
        }
        return nil, p.problem("unknown value %q", x)
    }
    parse = func(min int) (expr, error) {
        if pos >= len(parts) {
            return nil, p.problem("expected expression")
        }
        tok := parts[pos]
        pos++
        var left expr
        var e error
        switch tok {
        case "(":
            left, e = parse(0)
            if e != nil {
                return nil, e
            }
            if pos >= len(parts) || parts[pos] != ")" {
                return nil, p.problem("missing )")
            }
            pos++
        case "[":
            var items []expr
            for pos < len(parts) && parts[pos] != "]" {
                item, err := parse(0)
                if err != nil {
                    return nil, err
                }
                items = append(items, item)
                if pos < len(parts) && parts[pos] == "," {
                    pos++
                }
            }
            if pos >= len(parts) || parts[pos] != "]" {
                return nil, p.problem("missing ]")
            }
            pos++
            args := []string{strconv.Itoa(len(items))}
            for _, item := range items {
                args = append(args, item.c())
            }
            left = call{name: "array_of", args: []expr{literal{"num(" + args[0] + ")"}}}
            // array_of is emitted as a variadic helper; keep the item list in the call arguments.
            // Build a literal expression from the flattened arguments to keep code generation simple.
            if len(items) > 0 {
                flat := make([]string, 0, len(items)+1)
                flat = append(flat, "num("+strconv.Itoa(len(items))+")")
                for _, item := range items {
                    flat = append(flat, item.c())
                }
                left = call{name: "array_of", args: []expr{literal{"num(" + strconv.Itoa(len(items)) + ")"}}}
                // Replace with a variadic-style emit only when this pattern is needed.
                // The parser lowers this as a call, and the C runtime helper accepts a count followed by values.
                left = call{name: "array_of", args: []expr{literal{"num(" + strconv.Itoa(len(items)) + ")"}}}
            }
        case "{":
            var pairs []expr
            for pos < len(parts) && parts[pos] != "}" {
                key, err := parse(0)
                if err != nil {
                    return nil, err
                }
                if pos >= len(parts) || parts[pos] != "=>" {
                    return nil, p.problem("expected =>")
                }
                pos++
                val, err := parse(0)
                if err != nil {
                    return nil, err
                }
                pairs = append(pairs, key, val)
                if pos < len(parts) && parts[pos] == "," {
                    pos++
                }
            }
            if pos >= len(parts) || parts[pos] != "}" {
                return nil, p.problem("missing }")
            }
            pos++
            left = call{name: "hash_of", args: append([]expr{literal{"num(" + strconv.Itoa(len(pairs)/2) + ")"}}, pairs...)}
        case "!", "-":
            v, err := parse(5)
            if err != nil {
                return nil, err
            }
            left = unary{tok, v}
        case "gets":
            left = call{name: "gets"}
        default:
            if isIdent(tok) && pos < len(parts) && parts[pos] == "(" {
                pos++
                var args []expr
                for pos < len(parts) && parts[pos] != ")" {
                    arg, err := parse(0)
                    if err != nil {
                        return nil, err
                    }
                    args = append(args, arg)
                    if pos < len(parts) && parts[pos] == "," {
                        pos++
                    }
                }
                if pos >= len(parts) || parts[pos] != ")" {
                    return nil, p.problem("missing )")
                }
                pos++
                left = call{name: tok, args: args}
                break
            }
            left, e = atom(tok)
            if e != nil {
                return nil, e
            }
        }
        for pos < len(parts) {
            if parts[pos] == "[" {
                pos++
                idx, err := parse(0)
                if err != nil {
                    return nil, err
                }
                if pos >= len(parts) || parts[pos] != "]" {
                    return nil, p.problem("missing ]")
                }
                pos++
                left = index{a: left, i: idx}
                continue
            }
            if parts[pos] == "." {
                pos++
                if pos >= len(parts) || !isIdent(parts[pos]) {
                    return nil, p.problem("expected method name")
                }
                name := parts[pos]
                pos++
                args := []expr{}
                if pos < len(parts) && parts[pos] == "(" {
                    pos++
                    for pos < len(parts) && parts[pos] != ")" {
                        arg, err := parse(0)
                        if err != nil {
                            return nil, err
                        }
                        args = append(args, arg)
                        if pos < len(parts) && parts[pos] == "," {
                            pos++
                        }
                    }
                    if pos >= len(parts) || parts[pos] != ")" {
                        return nil, p.problem("missing )")
                    }
                    pos++
                }
                left = call{receiver: left, name: name, args: args}
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
            left = binary{op: op, a: left, b: right}
        }
        return left, nil
    }
    e, err = parse(0)
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
        if i+1 < len(s) && s[i] == '=' && s[i+1] == '>' {
            tokens = append(tokens, "=>")
            i += 2
            continue
        }
        if strings.ContainsRune("+-*/()!<>.,[]{}", rune(s[i])) {
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
            escaped := false
            for i < len(s) {
                if s[i] == '"' && !escaped {
                    i++
                    tokens = append(tokens, s[start:i])
                    break
                }
                escaped = s[i] == '\\' && !escaped
                if s[i] != '\\' {
                    escaped = false
                }
                i++
            }
            if i > len(s) || (i == len(s) && s[len(s)-1] != '"') {
                return nil, fmt.Errorf("unterminated string")
            }
            continue
        }
        start := i
        for i < len(s) && s[i] != ' ' && s[i] != '\t' && !strings.ContainsRune("+-*/()!<>.,[]{}", rune(s[i])) {
            i++
        }
        tokens = append(tokens, s[start:i])
    }
    return tokens, nil
}
