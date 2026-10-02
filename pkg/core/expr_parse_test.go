package core

import (
	"math"
	"strings"
	"testing"
)

func TestNewFromStringNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"3.14", 3.14},
		{"-5", -5},
		{"+5", 5},
		{"1e2", 100},
		{"1.5e-1", 0.15},
	}
	for _, tc := range cases {
		e, err := NewFromString(tc.in)
		if err != nil {
			t.Fatalf("%q: parse error: %v", tc.in, err)
		}
		got := e.Eval(nil, nil)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("%q = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewFromStringBinary(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"1+2", 3},
		{"10-3", 7},
		{"4*5", 20},
		{"10/4", 2.5},
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"2*3+4", 10},
		{"2^10", 1024},
		{"2^3^2", 512}, // right-assoc: 2^(3^2)
		{"1-2-3", -4},  // left-assoc: (1-2)-3
	}
	for _, tc := range cases {
		e, err := NewFromString(tc.in)
		if err != nil {
			t.Fatalf("%q: parse error: %v", tc.in, err)
		}
		got := e.Eval(nil, nil)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("%q = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewFromStringConstants(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"pi", math.Pi},
		{"PI", math.Pi},
		{"e", math.E},
		{"2*pi", 2 * math.Pi},
		{"pi/2", math.Pi / 2},
	}
	for _, tc := range cases {
		e, err := NewFromString(tc.in)
		if err != nil {
			t.Fatalf("%q: parse error: %v", tc.in, err)
		}
		got := e.Eval(nil, nil)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("%q = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewFromStringFunctions(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"sqrt(2)", math.Sqrt2},
		{"sq(3)", 9},
		{"sin(0)", 0},
		{"cos(0)", 1},
		{"tan(0)", 0},
		{"asin(0)", 0},
		{"acos(1)", 0},
		{"atan(0)", 0},
		{"atan2(0, 1)", 0},
		{"atan2(1, 0)", 90},
		{"sin(30)", 0.5},
		{"cos(60)", 0.5},
		{"tan(45)", 1},
		{"asin(0.5)", 30},
		{"acos(0.5)", 60},
		{"atan(1)", 45},
		{"hypot(3, 4)", 5},
		{"exp(0)", 1},
		{"ln(e)", 1},
		{"log(100)", 2},
		{"abs(-7)", 7},
		{"floor(1.7)", 1},
		{"ceil(1.2)", 2},
		{"round(1.5)", 2},
		{"min(3, 1, 2)", 1},
		{"max(3, 1, 2)", 3},
		{"min(5)", 5},
	}
	for _, tc := range cases {
		e, err := NewFromString(tc.in)
		if err != nil {
			t.Fatalf("%q: parse error: %v", tc.in, err)
		}
		got := e.Eval(nil, nil)
		if math.Abs(got-tc.want) > 1e-6 {
			t.Fatalf("%q = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewFromStringVariables(t *testing.T) {
	e, err := NewFromString("2*pi*r")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	vars := map[string]float64{"r": 5}
	got := e.Eval(nil, vars)
	want := 2 * math.Pi * 5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("2*pi*r with r=5 = %v, want %v", got, want)
	}
}

func TestNewFromStringNested(t *testing.T) {
	e, err := NewFromString("sqrt(sq(3) + sq(4))")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := e.Eval(nil, nil)
	if math.Abs(got-5) > 1e-9 {
		t.Fatalf("sqrt(3²+4²) = %v, want 5", got)
	}
}

func TestNewFromStringErrors(t *testing.T) {
	bad := []string{
		"1 +",         // trailing op
		"(1",          // unclosed paren
		"foo(",        // unclosed function
		"bogus(x, y)", // unknown function
		"min()",       // min requires at least one arg
	}
	for _, in := range bad {
		if _, err := NewFromString(in); err == nil {
			t.Errorf("%q: expected error, got nil", in)
		}
	}
}

func TestNewFromStringFoldConstants(t *testing.T) {
	e, _ := NewFromString("2 + 3*4")
	folded := e.FoldConstants()
	if folded.Op != OpConstant || folded.Value != 14 {
		t.Fatalf("FoldConstants did not simplify 2+3*4, got Op=%d Value=%v", folded.Op, folded.Value)
	}
}

// TestNewFromStringErrorMessages verifies Goal Item 9: parse
// errors include the position and the offending token.
func TestNewFromStringErrorMessages(t *testing.T) {
	cases := []struct {
		in       string
		contains string
	}{
		{"1 +", "unexpected"},
		{"(1", "expected ')'"},
		{"bogus(x, y)", "unknown function"},
		{"atan2(1)", "2 arguments"},
		{"hypot(1)", "2 arguments"},
	}
	for _, tc := range cases {
		_, err := NewFromString(tc.in)
		if err == nil {
			t.Errorf("%q: expected error, got nil", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.contains) {
			t.Errorf("%q: error %q does not contain %q", tc.in, err.Error(), tc.contains)
		}
	}
}
