package tkgui

import (
	"io/fs"
	"strings"

	"github.com/takigo/takigo/bind"
	"github.com/takigo/takigo/geometry/pack"
	"github.com/takigo/takigo/screenunit"
	"github.com/takigo/takigo/ttk"
	"github.com/takigo/takigo/widget"

	"github.com/telesapr/telesapr/pkg/internal/markdown"
)

// guideContents is the guide's contents on the left of its window: the
// sections and pages of the site's nav, and under the page shown its
// headings. The sections fold; the whole pane can be hidden.
type guideContents struct {
	g     *guideWindow
	frame *ttk.Frame
	tree  *ttk.Treeview
	shown bool
	// filled is set once the pages' entries are in the tree.
	filled bool
	// headings are the items of the shown page's headings.
	headings []string
	// syncing is set while the tree is made to follow the page, so that
	// its selecting does not show the page again.
	syncing bool
}

// The prefixes of the items' identifiers: a section, a page, a heading
// of the page shown.
const (
	contentsSection = "s:"
	contentsPage    = "p:"
	contentsHeading = "h:"
)

// guideContentsWidth is the pane's width, in pixels.
const guideContentsWidth = 230

func newGuideContents(g *guideWindow) *guideContents {
	p := g.p
	c := &guideContents{g: g, shown: true}
	c.frame = ttk.NewFrame(g.panes, p.newName("contents"))
	scroll := ttk.NewScrollbar(c.frame, p.newName("contentsscroll"), ttk.ScrollbarOrient(ttk.Vertical))
	_ = pack.Pack(scroll, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	c.tree = ttk.NewTreeview(c.frame, p.newName("tree"), ttk.TreeviewShow("tree"),
		ttk.TreeviewSelectMode(ttk.TreeSelectBrowse),
		ttk.TreeviewYScrollCommand(func(first, last float64) { scroll.Set(first, last) }))
	c.tree.ColumnConfigure("#0", ttk.ColWidth(screenunit.Px(guideContentsWidth)))
	_ = scroll.Configure(ttk.ScrollbarCommand(widget.ScrollY(c.tree)))
	_ = pack.Pack(c.tree, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	_ = g.panes.Add(c.frame, ttk.PaneWeight(0))
	_ = p.app.Bind().BindWindow(c.tree, "<<"+ttk.TreeviewSelectEvent+">>", func(*bind.EventData) bool {
		c.selected()
		return false
	})
	return c
}

// fill puts the entries of the guide's nav.yml in the tree, once.
func (c *guideContents) fill(pages fs.FS) {
	if c.filled {
		return
	}
	src, err := fs.ReadFile(pages, "nav.yml")
	if err != nil {
		// Without contents the pane is left empty and hidden.
		c.setShown(false)
		return
	}
	c.filled = true
	var add func(parent string, entries []markdown.Entry)
	add = func(parent string, entries []markdown.Entry) {
		for _, e := range entries {
			if e.Page != "" {
				c.tree.Insert(parent, -1, ttk.ItemID(contentsPage+e.Page), ttk.ItemText(e.Title))
				continue
			}
			id := c.tree.Insert(parent, -1, ttk.ItemID(contentsSection+e.Title), ttk.ItemText(e.Title),
				ttk.ItemOpen(true))
			add(id, e.Children)
		}
	}
	add("", markdown.ParseNav(string(src)))
}

// showPage makes the tree follow the page shown: its item selected and
// in view, its section open, and its second-level headings under it.
func (c *guideContents) showPage(page string, blocks []markdown.Block) {
	c.tree.Delete(c.headings...)
	c.headings = c.headings[:0]
	id := contentsPage + page
	if !c.tree.Exists(id) {
		return
	}
	for _, b := range blocks {
		if b.Kind != markdown.Heading || b.Level != 2 {
			continue
		}
		h := c.tree.Insert(id, -1, ttk.ItemID(contentsHeading+markdown.Anchor(b.Spans)),
			ttk.ItemText(markdown.Plain(b.Spans)))
		if h != "" {
			c.headings = append(c.headings, h)
		}
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	for parent := c.tree.Parent(id); parent != ""; parent = c.tree.Parent(parent) {
		c.tree.SetItemOpen(parent, true)
	}
	c.tree.SetItemOpen(id, true)
	c.tree.SelectionSet(id)
	c.tree.SetFocus(id)
	c.tree.See(id)
}

// selected shows what was picked in the tree: a page, or a heading of
// the page shown. A section only folds.
func (c *guideContents) selected() {
	if c.syncing {
		return
	}
	sel := c.tree.Selection()
	if len(sel) == 0 {
		return
	}
	switch id := sel[0]; {
	case strings.HasPrefix(id, contentsPage):
		page := strings.TrimPrefix(id, contentsPage)
		if page != c.g.page {
			c.g.show(page, true)
		}
	case strings.HasPrefix(id, contentsHeading):
		c.g.show("#"+strings.TrimPrefix(id, contentsHeading), false)
	}
}

// toggle hides the pane, or shows it again.
func (c *guideContents) toggle() { c.setShown(!c.shown) }

func (c *guideContents) setShown(shown bool) {
	if shown == c.shown {
		return
	}
	c.shown = shown
	// The window keeps its size: without one set, a toplevel follows what
	// its content asks for, and taking the pane out or putting it back
	// would shrink it to that. The text takes up the difference. The size
	// is recorded as a dragged border records it, not set with a geometry,
	// which would also move the window to the position takigo last knew.
	if top := c.g.top.Window(); top.Width > 1 && top.Height > 1 {
		c.g.top.WmInfo.UserW, c.g.top.WmInfo.UserH = top.Width, top.Height
	}
	if shown {
		_ = c.g.panes.Insert(0, c.frame, ttk.PaneWeight(0))
		return
	}
	c.g.panes.Forget(c.frame)
}
