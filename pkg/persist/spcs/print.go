package spcs

import (
	"strconv"
	"strings"
)

// Print writes a tree back as text. Top-level statements are separated
// by a blank line before every block and wherever the statement name
// changes, so runs of the same field, such as values, stay together.
func Print(f *File) []byte {
	var b strings.Builder
	prev := ""
	for i, n := range f.Nodes {
		if i > 0 && (n.Block || n.Name != prev || f.Nodes[i-1].Block) {
			b.WriteByte('\n')
		}
		printNode(&b, n, 0)
		prev = n.Name
	}
	return []byte(b.String())
}

func printNode(b *strings.Builder, n *Node, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
	b.WriteString(n.Name)
	for _, v := range n.Args {
		b.WriteByte(' ')
		if v.Quoted {
			b.WriteString(strconv.Quote(v.Text))
		} else {
			b.WriteString(v.Text)
		}
	}
	switch {
	case n.HasExpr:
		b.WriteString(" = ")
		b.WriteString(n.Expr)
		b.WriteByte('\n')
	case n.Block:
		b.WriteString(" {\n")
		for _, c := range n.Children {
			printNode(b, c, depth+1)
		}
		for i := 0; i < depth; i++ {
			b.WriteString("  ")
		}
		b.WriteString("}\n")
	default:
		b.WriteByte('\n')
	}
}
