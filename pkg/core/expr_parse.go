package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// NewFromString parses a textual expression and returns the
// equivalent *Expr. Goal Item 9: error messages include the byte
// position of the failure and the offending character.
func NewFromString(s string) (*Expr, error) {
	p := &prattParser{src: s}
	p.next()
	e, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.kind != tokEOF {
		return nil, fmt.Errorf("expr: unexpected %q at position %d in %q", p.lit, p.pos, s)
	}
	return e, nil
}

// tokKind enumerates Pratt-parser token kinds.
type tokKind int

const (
	tokEOF tokKind = iota
	tokNumber
	tokIdent
	tokOp
	tokLParen
	tokRParen
	tokComma
)

type token struct {
	kind tokKind
	lit  string
	num  float64
}

type prattParser struct {
	src  string
	pos  int
	cur  token
	kind tokKind
	lit  string
}

// next advances to the next token.
func (p *prattParser) next() {
	p.pos = skipSpace(p.src, p.pos)
	if p.pos >= len(p.src) {
		p.kind = tokEOF
		p.lit = ""
		p.cur = token{kind: tokEOF}
		return
	}
	c := rune(p.src[p.pos])
	switch {
	case c == '(':
		p.kind = tokLParen
		p.lit = "("
		p.cur = token{kind: tokLParen, lit: "("}
		p.pos++
	case c == ')':
		p.kind = tokRParen
		p.lit = ")"
		p.cur = token{kind: tokRParen, lit: ")"}
		p.pos++
	case c == ',':
		p.kind = tokComma
		p.lit = ","
		p.cur = token{kind: tokComma, lit: ","}
		p.pos++
	case isOpChar(c):
		p.readOp()
	case isDigit(c) || (c == '.' && p.pos+1 < len(p.src) && isDigit(rune(p.src[p.pos+1]))):
		p.readNumber()
	default:
		p.readIdent()
	}
}

// readOp reads one operator character: every operator is one, and
// reading a run of them would make "2*(3)" and "2*-3" one token.
func (p *prattParser) readOp() {
	start := p.pos
	p.pos++
	p.lit = p.src[start:p.pos]
	p.kind = tokOp
	p.cur = token{kind: tokOp, lit: p.lit}
}

func (p *prattParser) readNumber() {
	start := p.pos
	hasDot := false
	hasExp := false
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if isDigit(rune(c)) {
			p.pos++
			continue
		}
		if c == '.' && !hasDot && !hasExp {
			hasDot = true
			p.pos++
			continue
		}
		if (c == 'e' || c == 'E') && !hasExp {
			hasExp = true
			p.pos++
			if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
				p.pos++
			}
			continue
		}
		break
	}
	s := p.src[start:p.pos]
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Fall back: clamp to 0 and continue.
		v = 0
	}
	p.lit = s
	p.kind = tokNumber
	p.cur = token{kind: tokNumber, lit: s, num: v}
}

func (p *prattParser) readIdent() {
	start := p.pos
	for p.pos < len(p.src) && (isIdent(rune(p.src[p.pos])) || isDigit(rune(p.src[p.pos]))) {
		p.pos++
	}
	p.lit = p.src[start:p.pos]
	p.kind = tokIdent
	p.cur = token{kind: tokIdent, lit: p.lit}
}

// Precedence values for binary operators.
func binOpPrec(op string) (int, bool) {
	switch op {
	case "+", "-":
		return 1, true
	case "*", "/":
		return 2, true
	case "^":
		return 4, true
	}
	return 0, false
}

// parseExpr implements Pratt's recursive-descent. minPrec is the
// minimum precedence of operators to consume at this level.
func (p *prattParser) parseExpr(minPrec int) (*Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.kind == tokOp {
		prec, ok := binOpPrec(p.lit)
		if !ok || prec < minPrec {
			break
		}
		op := p.lit
		p.next()
		// Right-associative for ^: parseExpr(prec) not parseExpr(prec+1)
		nextMin := prec + 1
		if op == "^" {
			nextMin = prec
		}
		right, err := p.parseExpr(nextMin)
		if err != nil {
			return nil, err
		}
		left = makeBinary(op, left, right)
	}
	return left, nil
}

// parseUnary handles unary +/- and delegates to parsePrimary.
func (p *prattParser) parseUnary() (*Expr, error) {
	if p.kind == tokOp && (p.lit == "-" || p.lit == "+") {
		op := p.lit
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if op == "-" {
			return inner.Negate(), nil
		}
		return inner, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers, identifiers, constants, and function calls.
func (p *prattParser) parsePrimary() (*Expr, error) {
	switch p.kind {
	case tokNumber:
		e := NewConstant(p.cur.num)
		p.next()
		return e, nil
	case tokLParen:
		p.next()
		e, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if p.kind != tokRParen {
			return nil, fmt.Errorf("expr: expected ')' at position %d in %q, got %q", p.pos, p.src, p.lit)
		}
		p.next()
		return e, nil
	case tokIdent:
		name := p.lit
		p.next()
		// Constants
		switch name {
		case "pi", "PI":
			return NewConstant(math.Pi), nil
		case "e", "E":
			return NewConstant(math.E), nil
		}
		// Function call?
		if p.kind == tokLParen {
			return p.parseFunctionCall(name)
		}
		// Otherwise a variable.
		return NewVariable(name), nil
	case tokOp, tokRParen, tokComma, tokEOF:
		return nil, fmt.Errorf("expr: unexpected %q at position %d in %q", p.lit, p.pos, p.src)
	}
	return nil, fmt.Errorf("expr: internal error")
}

// parseFunctionCall reads `name(args...)`.
func (p *prattParser) parseFunctionCall(name string) (*Expr, error) {
	if p.kind != tokLParen {
		return nil, fmt.Errorf("expr: expected '(' after %q in function call", name)
	}
	p.next() // consume '('
	var args []*Expr
	if p.kind != tokRParen {
		for {
			a, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if p.kind == tokComma {
				p.next()
				continue
			}
			break
		}
	}
	if p.kind != tokRParen {
		return nil, fmt.Errorf("expr: expected ')' to close %q(...) call", name)
	}
	p.next() // consume ')'
	return makeFunction(name, args)
}

// makeFunction constructs the appropriate *Expr for a function call.
func makeFunction(name string, args []*Expr) (*Expr, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("expr: function %q requires at least one argument", name)
	}
	switch name {
	case "sqrt":
		return args[0].Sqrt(), nil
	case "sq":
		return args[0].Square(), nil
	// Angles a user types are in degrees, as in C++ ExprParser::Reduce
	// and in stored expressions.
	case "sin":
		return args[0].Times(NewConstant(math.Pi / 180)).Sin(), nil
	case "cos":
		return args[0].Times(NewConstant(math.Pi / 180)).Cos(), nil
	case "tan":
		return args[0].Times(NewConstant(math.Pi / 180)).Tan(), nil
	case "asin":
		return args[0].ASin().Times(NewConstant(180 / math.Pi)), nil
	case "acos":
		return args[0].ACos().Times(NewConstant(180 / math.Pi)), nil
	case "atan":
		return args[0].ATan().Times(NewConstant(180 / math.Pi)), nil
	case "atan2":
		if len(args) != 2 {
			return nil, fmt.Errorf("expr: function %q expects 2 arguments, got %d", name, len(args))
		}
		return args[0].ATan2(args[1]).Times(NewConstant(180 / math.Pi)), nil
	case "hypot":
		if len(args) != 2 {
			return nil, fmt.Errorf("expr: function %q expects 2 arguments, got %d", name, len(args))
		}
		return args[0].Hypot(args[1]), nil
	case "exp":
		return args[0].Exp(), nil
	case "ln":
		return args[0].Ln(), nil
	case "log":
		return args[0].Log(), nil
	case "abs":
		return args[0].Abs(), nil
	case "floor":
		return args[0].Floor(), nil
	case "ceil":
		return args[0].Ceil(), nil
	case "round":
		return args[0].Round(), nil
	case "min":
		return MinExpr(args...), nil
	case "max":
		return MaxExpr(args...), nil
	}
	return nil, fmt.Errorf("expr: unknown function %q", name)
}

// makeBinary builds the appropriate *Expr for a binary op.
func makeBinary(op string, a, b *Expr) *Expr {
	switch op {
	case "+":
		return a.Plus(b)
	case "-":
		return a.Minus(b)
	case "*":
		return a.Times(b)
	case "/":
		return a.Div(b)
	case "^":
		return a.Pow(b)
	}
	return nil
}

func skipSpace(s string, i int) int {
	for i < len(s) && unicode.IsSpace(rune(s[i])) {
		i++
	}
	return i
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }
func isOpChar(r rune) bool {
	return strings.ContainsRune("+-*/^=<>(),", r)
}
func isIdent(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// Variables returns the names the expression refers to, each once, in
// the order they first appear.
func (e *Expr) Variables() []string {
	var names []string
	seen := map[string]bool{}
	var walk func(e *Expr)
	walk = func(e *Expr) {
		if e == nil {
			return
		}
		if e.Op == OpVariable && !seen[e.VarID] {
			seen[e.VarID] = true
			names = append(names, e.VarID)
		}
		walk(e.Left)
		walk(e.Right)
		for _, a := range e.Args {
			walk(a)
		}
	}
	walk(e)
	return names
}

// RenameVariable rewrites the expression text s with every use of the
// name from replaced by to, leaving the rest of the text as it was:
// function names and other names are untouched.
func RenameVariable(s, from, to string) string {
	p := &prattParser{src: s}
	var b strings.Builder
	last := 0
	for {
		start := skipSpace(s, p.pos)
		p.next()
		if p.kind == tokEOF || p.pos == start {
			break
		}
		if p.kind == tokIdent && p.lit == from {
			if after := skipSpace(s, p.pos); after >= len(s) || s[after] != '(' {
				b.WriteString(s[last:start])
				b.WriteString(to)
				last = p.pos
			}
		}
	}
	b.WriteString(s[last:])
	return b.String()
}

// ValidVariableName reports whether name can name a parameter in an
// expression: a letter or underscore, then letters, digits and
// underscores, and none of the names the parser reserves for its
// constants and functions.
func ValidVariableName(name string) bool {
	if name == "" || !isIdent(rune(name[0])) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isIdent(rune(name[i])) && !isDigit(rune(name[i])) {
			return false
		}
	}
	switch name {
	case "pi", "PI", "e", "E":
		return false
	}
	if _, err := makeFunction(name, []*Expr{NewConstant(0), NewConstant(0)}); err == nil ||
		!strings.Contains(err.Error(), "unknown function") {
		return false
	}
	return true
}
