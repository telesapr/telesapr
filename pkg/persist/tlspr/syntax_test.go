package tlspr

import (
	"errors"
	"strings"
	"testing"
)

func TestParsePrintRoundTrip(t *testing.T) {
	src := `telesapr 1

units mm

group 0x2 sketch-in-plane "a \"name\"\twith\\escapes é" {
  order 1
  copies = 3
  predef {
    q 1 0 0 0
  }
  visible
}

value 0x1 1.5
value 0x2 -0
`
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Print(f)); got != src {
		t.Errorf("printed\n%s\nwant\n%s", got, src)
	}
	g := f.Nodes[2]
	if g.Name != "group" || !g.Block || len(g.Args) != 3 || !g.Args[2].Quoted ||
		g.Args[2].Text != "a \"name\"\twith\\escapes é" {
		t.Errorf("group node %+v", g)
	}
	if c := g.Children[1]; !c.HasExpr || c.Expr != "3" || c.ExprPos != (Pos{7, 10}) {
		t.Errorf("expression node %+v", c)
	}
	if p := g.Children[2]; p.Pos != (Pos{8, 3}) || len(p.Children) != 1 {
		t.Errorf("predef node %+v", p)
	}
}

func TestParseLexicalDetails(t *testing.T) {
	src := "# a comment before the header\r\n" +
		"telesapr 1 # trailing comment\r\n" +
		"\r\n" +
		"group 0x1 sketch-3d \"has # inside\" { # comment after the brace\n" +
		"  depth = wall * 10   # expression with a comment\n" +
		"}\n"
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Nodes) != 2 {
		t.Fatalf("got %d statements", len(f.Nodes))
	}
	g := f.Nodes[1]
	if g.Args[2].Text != "has # inside" {
		t.Errorf("string %q", g.Args[2].Text)
	}
	if e := g.Children[0]; e.Expr != "wall * 10" {
		t.Errorf("expression %q", e.Expr)
	}
	if _, err := Parse([]byte("telesapr 1")); err != nil {
		t.Errorf("no final line end: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, c := range []struct {
		src, want string
		pos       Pos
	}{
		{"", "the file is empty", Pos{1, 1}},
		{"# only a comment\n", "the file is empty", Pos{1, 1}},
		{"group 0x1 {\n}\n", "not a TeleSAPR file", Pos{1, 1}},
		{"telesapr 1 {\n}\n", "the header is not a block", Pos{1, 1}},
		{"telesapr 1\ngroup {\n", "block 'group' is not closed", Pos{2, 1}},
		{"telesapr 1\n}\n", "'}' with no block open", Pos{2, 1}},
		{"telesapr 1\ngroup {\n} x\n", "'}' must stand on a line of its own", Pos{3, 3}},
		{"telesapr 1\ngroup { x\n}\n", "nothing may follow '{'", Pos{2, 9}},
		{"telesapr 1\ngroup x }\n", "'}' must stand on a line of its own", Pos{2, 9}},
		{"telesapr 1\n\"name\" 1\n", "a statement must start with a name", Pos{2, 1}},
		{"telesapr 1\n{\n", "a statement must start with a name", Pos{2, 1}},
		{"telesapr 1\nx = \n", "expected an expression after '='", Pos{2, 3}},
		{"telesapr 1\nx = # nothing\n", "expected an expression after '='", Pos{2, 3}},
		{"telesapr 1\nx \"open\n", "string not closed on its line", Pos{2, 3}},
		{"telesapr 1\nx \"bad \\q escape\"\n", "bad string", Pos{2, 3}},
		{"telesapr 1\nx\ry\n", "carriage return inside a line", Pos{2, 2}},
	} {
		_, err := Parse([]byte(c.src))
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%q: error %v, want a syntax error", c.src, err)
			continue
		}
		if !strings.Contains(e.Msg, c.want) || e.Pos != c.pos {
			t.Errorf("%q: %v, want %q at %v", c.src, err, c.want, c.pos)
		}
	}
}

func TestErrorString(t *testing.T) {
	e := &Error{Pos{3, 7}, "boom"}
	if e.Error() != "3:7: boom" {
		t.Errorf("got %q", e.Error())
	}
}

func TestPrintSeparatesStatements(t *testing.T) {
	f := &File{Nodes: []*Node{
		field("telesapr", Word("1")),
		field("value", Word("0x1"), Word("1")),
		field("value", Word("0x2"), Word("2")),
		{Name: "group", Block: true},
		field("value", Word("0x3"), Word("3")),
		{Name: "x", HasExpr: true, Expr: "1 + 2"},
	}}
	want := "telesapr 1\n\nvalue 0x1 1\nvalue 0x2 2\n\ngroup {\n}\n\nvalue 0x3 3\n\nx = 1 + 2\n"
	if got := string(Print(f)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
