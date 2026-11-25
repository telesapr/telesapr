package tkgui

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/takigo/takigo/bind"
	"github.com/takigo/takigo/geometry/pack"
	tkimage "github.com/takigo/takigo/image"
	"github.com/takigo/takigo/screenunit"
	"github.com/takigo/takigo/ttk"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/text"
	"github.com/takigo/takigo/widget/text/richtext"
	"github.com/takigo/takigo/widget/toplevel"

	"github.com/telesapr/telesapr/pkg/internal/markdown"
	"github.com/telesapr/telesapr/pkg/version"
)

// guideWindow is the user's guide in a window of its own: a page of
// Markdown, made HTML and laid out by takigo's richtext in a text widget,
// its pictures in the text, its links followed in the window and a
// history to go back along.
type guideWindow struct {
	p      *Platform
	top    *toplevel.Toplevel
	frame  Window
	view   *text.TextWidget
	scroll *ttk.Scrollbar
	title  *ttk.Label
	// panes holds the contents on the left, which can be hidden, and the
	// page on the right.
	panes    *ttk.Panedwindow
	contents *guideContents

	pages  fs.FS
	online string
	page   string
	back   []string
	// photos keeps the pictures read, shrunk to the page's width.
	photos map[string]*tkimage.Photo
	style  guideStyle
}

// guideWidth and guideHeight are the size of the guide's text, in
// characters and lines.
const (
	guideWidth  = 96
	guideHeight = 40
)

// ShowGuide shows a page of the user's guide, opening its window the
// first time.
func (p *Platform) ShowGuide(pages fs.FS, page, online string) {
	if p.guide == nil {
		p.guide = p.newGuide()
	}
	g := p.guide
	g.pages, g.online = pages, online
	g.contents.fill(pages)
	g.show(page, true)
	g.frame.SetVisible(true)
	g.frame.present()
}

func (p *Platform) newGuide() *guideWindow {
	g := &guideWindow{p: p, photos: map[string]*tkimage.Photo{}}
	g.top = toplevel.New(p.app, p.newName("guide"), toplevel.Title("TeleSAPR User's Guide"))
	g.top.Hide()
	g.frame = Window{p: p, top: g.top, topWin: g.top.Window(), wmInfo: g.top.WmInfo, toolTop: g.top}
	setWindowIcon(g.frame.topWin)
	p.app.RegisterCloseHandler(g.frame.topWin.PlatformID, func() { g.frame.SetVisible(false) })
	g.style = newGuideStyle(p.app.FontRegistry().Families(), widget.PaletteFor(p.app).Dark)

	body := ttk.NewFrame(g.top, p.newName("body"), ttk.FramePadding(ttk.Padding{Left: 6, Top: 6, Right: 6, Bottom: 6}))
	_ = pack.Pack(body, pack.FillOpt(pack.FillBoth), pack.Expand(true))

	bar := ttk.NewFrame(body, p.newName("bar"))
	_ = pack.Pack(bar, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillX), pack.PadY(screenunit.Px(4)))
	for _, b := range []struct {
		text string
		do   func()
	}{
		{"☰ Contents", func() { g.contents.toggle() }},
		{"Home", func() { g.show("README.md", true) }},
		{"Back", g.goBack},
		{"Open Online", g.openOnline},
	} {
		button := ttk.NewButton(bar, p.newName("button"), ttk.ButtonText(b.text), ttk.ButtonCommand(b.do))
		_ = pack.Pack(button, pack.SideOpt(pack.Left), pack.PadX(screenunit.Px(2)))
	}
	g.title = ttk.NewLabel(bar, p.newName("title"), ttk.LabelText(""))
	_ = pack.Pack(g.title, pack.SideOpt(pack.Left), pack.PadX(screenunit.Px(12)))

	g.panes = ttk.NewPanedwindow(body, p.newName("panes"), ttk.PanedwindowOrient(ttk.Horizontal))
	_ = pack.Pack(g.panes, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	g.contents = newGuideContents(g)

	shown := ttk.NewFrame(g.panes, p.newName("shown"))
	_ = g.panes.Add(shown, ttk.PaneWeight(1))
	g.scroll = ttk.NewScrollbar(shown, p.newName("scroll"), ttk.ScrollbarOrient(ttk.Vertical))
	_ = pack.Pack(g.scroll, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	g.view = text.New(shown, p.newName("page"), text.Width(guideWidth), text.Height(guideHeight),
		text.FontOpt(g.style.font()), text.WrapModeOpt(text.WrapWord),
		text.Background(g.style.paper), text.Foreground(g.style.ink),
		text.UndoOpt(false), text.ReadOnly(true), text.PadXOpt(screenunit.Px(12)), text.PadYOpt(screenunit.Px(8)),
		text.YScrollCommand(func(first, last float64) { g.scroll.Set(first, last) }))
	_ = g.scroll.Configure(ttk.ScrollbarCommand(widget.ScrollY(g.view)))
	_ = pack.Pack(g.view, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))

	// Alt+Left goes back, as in a browser.
	_ = p.app.Bind().Bind(g.view.Window().PathName, "<Alt-Left>", func(*bind.EventData) bool {
		g.goBack()
		return true
	})
	return g
}

// show shows a page, and an anchor in it after a '#'; remember keeps
// the page left in the history.
func (g *guideWindow) show(target string, remember bool) {
	page, anchor, _ := strings.Cut(target, "#")
	if page == "" {
		page = g.page
	}
	if page != g.page {
		src, err := fs.ReadFile(g.pages, page)
		if err != nil {
			g.p.OpenInBrowser(g.onlineURL(page))
			return
		}
		if remember && g.page != "" {
			g.back = append(g.back, g.page)
		}
		g.page = page
		g.render(markdown.Parse(string(src)))
	}
	g.view.YViewMoveTo(0)
	if anchor != "" {
		g.scrollTo(anchor)
	}
}

func (g *guideWindow) goBack() {
	if len(g.back) == 0 {
		return
	}
	page := g.back[len(g.back)-1]
	g.back = g.back[:len(g.back)-1]
	g.show(page, false)
}

func (g *guideWindow) openOnline() { g.p.OpenInBrowser(g.onlineURL(g.page)) }

// onlineURL is a page's address on the guide's site, which makes
// fillet.md the directory fillet/.
func (g *guideWindow) onlineURL(page string) string {
	page = strings.TrimSuffix(page, ".md")
	if page == "README" {
		return g.online
	}
	return g.online + page + "/"
}

// follow follows a link of the page shown.
func (g *guideWindow) follow(href string) {
	switch {
	case strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:"):
		g.p.OpenInBrowser(href)
	case strings.HasPrefix(href, "#"):
		g.show(href, false)
	default:
		target, anchor, _ := strings.Cut(href, "#")
		target = path.Clean(path.Join(path.Dir(g.page), target))
		if !strings.HasSuffix(target, ".md") {
			// A file of the repository, as an example script.
			g.p.OpenInBrowser(repositoryFile(path.Join("doc/guide", target)))
			return
		}
		if anchor != "" {
			target += "#" + anchor
		}
		g.show(target, true)
	}
}

// render sets a page in the text: its Markdown made HTML, which takigo's
// richtext lays out with real tables, list markers and boxes behind code,
// and lays out again when the text's width changes.
func (g *guideWindow) render(blocks []markdown.Block) {
	_ = g.title.Configure(ttk.LabelText(g.page))
	g.contents.showPage(g.page, blocks)
	page := g.page
	err := richtext.Render(g.view, markdown.HTML(lettered(blocks), g.onlineURL(page)),
		richtext.BaseFont(g.style.font()),
		richtext.Stylesheet(g.style.css()),
		richtext.OnLink(g.follow),
		richtext.ImageResolver(func(src string) (widget.WidgetImage, error) {
			ph := g.photo(path.Clean(path.Join(path.Dir(page), src)))
			if ph == nil {
				return nil, fmt.Errorf("guide picture %s: %w", src, fs.ErrNotExist)
			}
			return ph, nil
		}))
	if err != nil {
		// What could be inserted is shown; the rest is left out.
		_ = g.view.Insert(g.view.EndIndex(), "\n"+err.Error())
	}
}

// scrollTo puts a heading of the page shown at the top. richtext marks
// each element's id with a name that ends in "#" and the id.
func (g *guideWindow) scrollTo(anchor string) {
	for _, m := range g.view.MarkNames() {
		if strings.HasPrefix(m, "richtext:") && strings.HasSuffix(m, ":#"+anchor) {
			if idx, err := g.view.Index(m); err == nil {
				g.view.YView(idx.Line)
			}
			return
		}
	}
}

// photo reads a picture of the guide at its own size; nil if it cannot
// be read. richtext fits it to the text's width each time it lays the
// page out, averaging pixels as it shrinks, so a picture follows the
// window down and up to its own size and no further.
func (g *guideWindow) photo(name string) *tkimage.Photo {
	if ph, ok := g.photos[name]; ok {
		return ph
	}
	f, err := g.pages.Open(name)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	rgba, err := decodePicture(f)
	if err != nil {
		return nil
	}
	ph := tkimage.NewPhoto(g.p.newName("guidepic"), rgba)
	g.photos[name] = ph
	return ph
}

// repositoryFile is a file of the repository on its site.
func repositoryFile(name string) string {
	return version.Repository + "/blob/master/" + strings.TrimPrefix(path.Clean(name), "/")
}
