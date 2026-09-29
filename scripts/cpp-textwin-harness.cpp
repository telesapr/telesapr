// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile), shows each text window screen for it, and prints the
// text window's rows: their tops, their text, and each cell's colours
// and link. The headless build has no window, so the text window gets
// a Cairo canvas for its bitmap font. Any further arguments are font
// files for TrueType text. pkg/ui/textwin_test.go runs the same
// scenarios, in the same order. See scripts/regen-textwin-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

static void PutUtf8(uint32_t c) {
    if(c < 0x80) {
        putchar((int)c);
    } else if(c < 0x800) {
        putchar(0xc0 | (c >> 6)); putchar(0x80 | (c & 0x3f));
    } else if(c < 0x10000) {
        putchar(0xe0 | (c >> 12)); putchar(0x80 | ((c >> 6) & 0x3f));
        putchar(0x80 | (c & 0x3f));
    } else {
        putchar(0xf0 | (c >> 18)); putchar(0x80 | ((c >> 12) & 0x3f));
        putchar(0x80 | ((c >> 6) & 0x3f)); putchar(0x80 | (c & 0x3f));
    }
}

// The cell's look and link, as the dump compares it: the data and link
// functions only matter for links, and the RGB background only for 'z'.
static std::string CellKey(int r, int c) {
    auto &m = SS.TW.meta[r][c];
    std::string key = ssprintf("%02x %02x", (unsigned char)m.fg, (unsigned char)m.bg);
    if(m.bg == 'z') {
        key += ssprintf(" %02x%02x%02x%02x", m.bgRgb.red, m.bgRgb.green, m.bgRgb.blue,
                        m.bgRgb.alpha);
    }
    key += ssprintf(" %d", m.link);
    if(m.link) {
        key += ssprintf(" %x %d %d", m.data, m.f ? 1 : 0, m.h ? 1 : 0);
    }
    return key;
}

static void DumpRows() {
    TextWindow &tw = SS.TW;
    for(int r = 0; r < tw.rows; r++) {
        int end = TextWindow::MAX_COLS;
        while(end > 0 && tw.text[r][end-1] == ' ') end--;
        // Skip the renderer's identification, which differs.
        std::string ascii;
        for(int c = 0; c < end; c++) ascii += (tw.text[r][c] < 0x80) ? (char)tw.text[r][c] : '?';
        if(ascii.find("gl vendor") != std::string::npos ||
           ascii.find("   renderer ") != std::string::npos ||
           ascii.find("   version  ") != std::string::npos) {
            printf("row %d (renderer)\n", tw.top[r]);
            continue;
        }
        printf("row %d ", tw.top[r]);
        for(int c = 0; c < end; c++) PutUtf8(tw.text[r][c]);
        printf("\n");
        int c0 = 0;
        while(c0 < TextWindow::MAX_COLS) {
            std::string key = CellKey(r, c0);
            int c1 = c0 + 1;
            while(c1 < TextWindow::MAX_COLS && CellKey(r, c1) == key) c1++;
            printf(" %d-%d %s\n", c0, c1 - 1, key.c_str());
            c0 = c1;
        }
    }
}

static void Dump(const char *name) {
    SS.TW.Show();
    printf("screen %s\n", name);
    DumpRows();
}

static void ShowScreen(const char *name, TextWindow::Screen screen) {
    SS.TW.shown.screen = screen;
    Dump(name);
}

static void Select(std::vector<hEntity> es, std::vector<hConstraint> cs) {
    SS.GW.ClearSelection();
    for(hEntity he : es) {
        GraphicsWindow::Selection sel = {};
        sel.entity = he;
        SS.GW.selection.Add(&sel);
    }
    for(hConstraint hc : cs) {
        GraphicsWindow::Selection sel = {};
        sel.constraint = hc;
        SS.GW.selection.Add(&sel);
    }
}

int main(int argc, char **argv) {
    if(argc < 2) return 2;
    for(int i = 2; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    SS.Init();
    if(!SS.LoadFromFile(Platform::Path::From(argv[1]))) return 1;
    SS.AfterNewFile();
    // As after File > Open, so linked files are named relative to it.
    SS.saveFile = Platform::Path::From(argv[1]).Expand(/*fromCurrentDirectory=*/true);
    SS.TW.canvas = std::make_shared<CairoPixmapRenderer>();
    // Without a window C++ doesn't zoom to fit, or generate the display
    // meshes: pin the view, and generate them as drawing would.
    SS.GW.scale     = 5;
    SS.GW.offset    = {0, 0, 0};
    SS.GW.projRight = {1, 0, 0};
    SS.GW.projUp    = {0, 1, 0};
    for(hGroup hg : SK.groupOrder) SK.GetGroup(hg)->GenerateDisplayItems();

    ShowScreen("groups", TextWindow::Screen::LIST_OF_GROUPS);

    std::vector<hGroup> groups;
    for(hGroup hg : SK.groupOrder) groups.push_back(hg);
    for(hGroup hg : groups) {
        SS.TW.shown.group = hg;
        ShowScreen(ssprintf("group %x", hg.v).c_str(), TextWindow::Screen::GROUP_INFO);
        SS.TW.shown.group = hg;
        ShowScreen(ssprintf("solve %x", hg.v).c_str(), TextWindow::Screen::GROUP_SOLVE_INFO);
    }

    // The group screens of the group types no fixture has, shown for the
    // extrusions.
    for(hGroup hg : groups) {
        Group *g = SK.GetGroup(hg);
        if(g->type != Group::Type::EXTRUDE) continue;
        Group saved = *g;
        struct { Group::Type type; Group::Subtype subtype; double valA, valB; } variants[] = {
            { Group::Type::LATHE,     Group::Subtype::ONE_SIDED,  0, 0 },
            { Group::Type::REVOLVE,   Group::Subtype::TWO_SIDED,  0, 0 },
            { Group::Type::HELIX,     Group::Subtype::ONE_SIDED,  0, 2.5 },
            { Group::Type::ROTATE,    Group::Subtype::ONE_SIDED,  3, 0 },
            { Group::Type::TRANSLATE, Group::Subtype::TWO_SIDED,  1, 0 },
            { Group::Type::EXTRUDE,   Group::Subtype::TWO_SKEWED, 0, 0 },
        };
        int i = 0;
        for(auto &v : variants) {
            g->type = v.type;
            g->subtype = v.subtype;
            g->valA = v.valA;
            g->valB = v.valB;
            g->skipFirst = (i % 2 == 1);
            g->booleanFailed = (i == 5);
            SS.TW.shown.group = hg;
            ShowScreen(ssprintf("variant %x %d", hg.v, i).c_str(), TextWindow::Screen::GROUP_INFO);
            i++;
        }
        *g = saved;
    }

    ShowScreen("configuration", TextWindow::Screen::CONFIGURATION);
    ShowScreen("styles", TextWindow::Screen::LIST_OF_STYLES);
    std::vector<hStyle> styles;
    for(Style &s : SK.style) styles.push_back(s.h);
    for(hStyle hs : styles) {
        SS.TW.shown.style = hs;
        ShowScreen(ssprintf("style %x", hs.v).c_str(), TextWindow::Screen::STYLE_INFO);
    }
    ShowScreen("view", TextWindow::Screen::EDIT_VIEW);
    ShowScreen("tangent", TextWindow::Screen::TANGENT_ARC);
    SS.tangentArcManual = !SS.tangentArcManual;
    ShowScreen("tangent manual", TextWindow::Screen::TANGENT_ARC);
    SS.tangentArcManual = !SS.tangentArcManual;

    SS.TW.shown.paste.times  = 3;
    SS.TW.shown.paste.trans  = Vector::From(1.5, -2, 0);
    SS.TW.shown.paste.theta  = 0.5;
    SS.TW.shown.paste.origin = Vector::From(0.25, 0.75, 0);
    SS.TW.shown.paste.scale  = -1.5;
    ShowScreen("paste", TextWindow::Screen::PASTE_TRANSFORMED);

    for(Constraint &c : SK.constraint) {
        if(!c.HasLabel() || c.reference) continue;
        SS.TW.stepDim.finish = c.valA * 2;
        SS.TW.stepDim.steps = 10;
        SS.TW.stepDim.isDistance =
            (c.type != Constraint::Type::ANGLE) &&
            (c.type != Constraint::Type::LENGTH_RATIO) &&
            (c.type != Constraint::Type::ARC_ARC_LEN_RATIO) &&
            (c.type != Constraint::Type::ARC_LINE_LEN_RATIO) &&
            (c.type != Constraint::Type::LENGTH_DIFFERENCE) &&
            (c.type != Constraint::Type::ARC_ARC_DIFFERENCE) &&
            (c.type != Constraint::Type::ARC_LINE_DIFFERENCE);
        SS.TW.shown.constraint = c.h;
        SS.TW.shown.screen = TextWindow::Screen::STEP_DIMENSION;
        SS.TW.Show();
        // Its "steps" link reads a data argument that ShowStepDimension
        // doesn't pass.
        for(int r = 0; r < SS.TW.rows; r++) {
            for(int col = 0; col < TextWindow::MAX_COLS; col++) SS.TW.meta[r][col].data = 0;
        }
        printf("screen stepdim\n");
        DumpRows();
        break;
    }
    SS.TW.shown.screen = TextWindow::Screen::LIST_OF_GROUPS;

    // A pending operation preempts the screens.
    SS.GW.pending.operation = GraphicsWindow::Pending::COMMAND;
    SS.GW.pending.description = "click first point of line segment";
    Dump("pending");
    SS.GW.pending.operation = GraphicsWindow::Pending::NONE;
    SS.GW.pending.description = NULL;

    // Descriptions of selections: each entity, each constraint, and runs
    // of two and three consecutive entities.
    std::vector<hEntity> entities;
    for(Entity &e : SK.entity) entities.push_back(e.h);
    const size_t maxEntities = 80;
    for(size_t i = 0; i < entities.size() && i < maxEntities; i++) {
        Select({ entities[i] }, {});
        Dump(ssprintf("entity %x", entities[i].v).c_str());
    }
    for(size_t i = 0; i + 1 < entities.size() && i < maxEntities; i++) {
        Select({ entities[i], entities[i+1] }, {});
        Dump(ssprintf("entities %x %x", entities[i].v, entities[i+1].v).c_str());
    }
    for(size_t i = 0; i + 2 < entities.size() && i < maxEntities; i++) {
        Select({ entities[i], entities[i+1], entities[i+2] }, {});
        Dump(ssprintf("entities %x %x %x", entities[i].v, entities[i+1].v,
                      entities[i+2].v).c_str());
    }
    std::vector<hConstraint> constraints;
    for(Constraint &c : SK.constraint) constraints.push_back(c.h);
    for(hConstraint hc : constraints) {
        Select({}, { hc });
        Dump(ssprintf("constraint %x", hc.v).c_str());
    }

    // A custom style, with a request in it, shown while the request is
    // selected.
    hStyle hs = { Style::CreateCustomStyle(/*rememberForUndo=*/false) };
    Style::Get(hs)->name = "custom";
    Style::Get(hs)->textOrigin = (Style::TextOrigin)((uint32_t)Style::TextOrigin::LEFT |
                                                     (uint32_t)Style::TextOrigin::TOP);
    for(Request &r : SK.request) {
        if(r.type == Request::Type::WORKPLANE) continue;
        r.style = hs;
        Select({ r.h.entity(0) }, {});
        break;
    }
    SS.TW.shown.style = hs;
    ShowScreen("custom style", TextWindow::Screen::STYLE_INFO);
    SS.GW.ClearSelection();
    ShowScreen("custom style info", TextWindow::Screen::STYLE_INFO);
    ShowScreen("styles with custom", TextWindow::Screen::LIST_OF_STYLES);

    // A redundant constraint, so the group fails to solve.
    for(Constraint &c : SK.constraint) {
        if(c.reference || c.type == Constraint::Type::COMMENT) continue;
        Constraint dup = c;
        dup.h = {};
        Constraint::AddConstraint(&dup, /*rememberForUndo=*/false);
        SS.GenerateAll(SolveSpaceUI::Generate::ALL);
        ShowScreen("failed groups", TextWindow::Screen::LIST_OF_GROUPS);
        SS.TW.shown.group = dup.group;
        ShowScreen("failed solve", TextWindow::Screen::GROUP_SOLVE_INFO);
        break;
    }
    return 0;
}
