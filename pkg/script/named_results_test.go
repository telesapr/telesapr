package script

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A function's named results start at their zero values on each call,
// and a composite literal assigned to one is what it returns, whether
// the script is a program, declarations or statements.
func TestNamedResults(t *testing.T) {
	t.Parallel()
	const funcs = `
type pair struct{ a, b int }

func some(x int) (n int, s []int) {
	if x > 0 {
		n = x
		s = append(s, x)
	}
	return n, s
}

func lit(x int) (p pair) {
	p = pair{x, x + 1}
	return
}

func inClosure(x int) (p pair, err error) {
	set := func() { p = pair{b: x} }
	set()
	return
}
`
	const calls = `
for _, x := range []int{3, 0, 5, 0} {
	n, s := some(x)
	fmt.Println(n, len(s))
}
for _, x := range []int{1, 2} {
	p := lit(x)
	q, _ := inClosure(x)
	fmt.Println(p.a, p.b, q.b)
}
`
	const want = "3 1\n0 0\n5 1\n0 0\n1 2 1\n2 3 2\n"
	for name, src := range map[string]string{
		"program":    "package main\n\nimport \"fmt\"\n" + funcs + "\nfunc main() {" + calls + "}\n",
		"statements": funcs + calls,
	} {
		var out bytes.Buffer
		if err := RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, name, src); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if out.String() != want {
			t.Errorf("%s printed\n%s\nwant\n%s", name, out.String(), want)
		}
	}
}

// The statements go on the lines that were there, so the interpreter's
// errors are on the script's own lines; source that doesn't parse, or
// names no results, is left alone.
func TestZeroNamedResultsSource(t *testing.T) {
	t.Parallel()
	src := "func f() (n int, _ string, p *T) {\n\tp = &T{}\n\tn = 1\n\treturn\n}\nfunc g() (r R) { r = R{1}; return }\n"
	got := zeroNamedResults(src)
	if want := strings.Count(src, "\n"); strings.Count(got, "\n") != want {
		t.Errorf("lines moved:\n%s", got)
	}
	for _, part := range []string{"{ n, p = *new(int), *new(*T);", "{ r = *new(R); r = (R{1}); return }"} {
		if !strings.Contains(got, part) {
			t.Errorf("no %q in\n%s", part, got)
		}
	}
	for _, same := range []string{"x := 1", "func f() int { return 1 }", "func ( {", "f := func() (n int) {"} {
		if got := zeroNamedResults(same); got != same {
			t.Errorf("%q became %q", same, got)
		}
	}
}
