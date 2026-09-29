// Drives C++ SolveSpace's user interface from a script, as a user would:
// mouse and keyboard events, menu items, context menus, dialogs and the
// editor, and prints what the interface shows. The headless build's
// platform (guinone.cpp) has no windows, menus or timers, so this file
// supplies recording ones; it is linked with --allow-multiple-definition
// so that its factories win. pkg/ui/uiscript_test.go runs the same
// scripts on the Go port. See scripts/regen-ui-golden.sh.
//
// Usage: harness <script> <temp file for saves> [font files...]
// The script's paths are relative to the current directory.
//
// Script lines (# starts a comment):
//   load <file>                   SS.Load
//   view default|rotated          pin the camera
//   move <x> <y> [L|M|R] [shift] [ctrl]
//   down|up|dbl <L|M|R> <x> <y> [shift] [ctrl]
//   scroll <x> <y> <delta> [shift] [ctrl]
//   leave
//   twmove/twdown/twup/twdbl      the same, on the text window
//   key <k> [shift] [ctrl]        k: a character, esc, del, tab, bs, ret,
//                                 space, or F1..F12
//   menu <label>|<label>|...      trigger a main menu item
//   popup <label>                 the item the next context menu picks
//   answer ok|yes|no|cancel       the next message dialog's answer
//   file <path>|-                 the next file dialog's answer
//   edit <text>                   finish the visible editor with text
//   dump                          interface state
//   hover                         the hovered item and the hover list
//   hovergrid <n>                 hover at n x n points, one line each
//   params                        every param's value
//   menus                         the main menu tree
//   tw                            the text window's rows
//   save                          save and print the file
// After every event, due timers fire and invalidated windows repaint,
// as the event loop does.
#include "solvespace.h"
#include <fstream>
#include <map>
#include <sstream>
using namespace SolveSpace;
using namespace SolveSpace::Platform;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

static std::string Esc(const std::string &s) {
    std::string r;
    for(char c : s) {
        if(c == '\n') r += "\\n"; else if(c == '\\') r += "\\\\"; else r += c;
    }
    return r;
}

static std::string G(double v) { return ssprintf("%.17g", v); }
static std::string V(const Vector &v) { return G(v.x) + " " + G(v.y) + " " + G(v.z); }

// Settings kept in memory.
class SettingsImplMemory final : public Settings {
public:
    std::map<std::string, uint32_t>    ints;
    std::map<std::string, double>      floats;
    std::map<std::string, std::string> strings;

    void FreezeInt(const std::string &key, uint32_t value) override { ints[key] = value; }
    uint32_t ThawInt(const std::string &key, uint32_t defaultValue) override {
        auto it = ints.find(key);
        return it == ints.end() ? defaultValue : it->second;
    }
    void FreezeFloat(const std::string &key, double value) override { floats[key] = value; }
    double ThawFloat(const std::string &key, double defaultValue) override {
        auto it = floats.find(key);
        return it == floats.end() ? defaultValue : it->second;
    }
    void FreezeString(const std::string &key, const std::string &value) override {
        strings[key] = value;
    }
    std::string ThawString(const std::string &key, const std::string &defaultValue) override {
        auto it = strings.find(key);
        return it == strings.end() ? defaultValue : it->second;
    }
};

namespace SolveSpace { namespace Platform {
SettingsRef GetSettings() {
    static std::shared_ptr<SettingsImplMemory> settings = std::make_shared<SettingsImplMemory>();
    return settings;
}
} }

// Timers fire in creation order, round by round, as the Go headless
// platform's RunTimers does; long ones (autosave) never fire.
class RecordingTimer;
static std::vector<std::shared_ptr<RecordingTimer>> timers;

class RecordingTimer final : public Timer {
public:
    bool     running = false;
    unsigned delay   = 0;
    void RunAfter(unsigned milliseconds) override { running = true; delay = milliseconds; }
};

namespace SolveSpace { namespace Platform {
TimerRef CreateTimer() {
    auto t = std::make_shared<RecordingTimer>();
    timers.push_back(t);
    return t;
}
} }

static bool RunTimers() {
    bool any = false;
    for(int round = 0; round < 1000; round++) {
        bool fired = false;
        size_t n = timers.size();
        for(size_t i = 0; i < n; i++) {
            auto t = timers[i];
            if(t->running && t->delay < 1000) {
                t->running = false;
                fired = any = true;
                if(t->onTimeout) t->onTimeout();
            }
        }
        if(!fired) break;
    }
    return any;
}

// Menus record their items.
class RecordingMenu;
class RecordingMenuItem final : public MenuItem {
public:
    std::string                    label;
    bool                           mnemonics = true;
    std::string                    accel;
    Indicator                      indicator = Indicator::NONE;
    bool                           enabled   = true;
    bool                           active    = false;
    bool                           separator = false;
    std::shared_ptr<RecordingMenu> subMenu;

    std::string Stripped() const {
        if(!mnemonics) return label;
        std::string r;
        for(char c : label) if(c != '&') r += c;
        return r;
    }
    void SetAccelerator(KeyboardEvent a) override { accel = AcceleratorDescription(a); }
    void SetIndicator(Indicator type) override { indicator = type; }
    void SetEnabled(bool e) override { enabled = e; }
    void SetActive(bool a) override { active = a; }
};

static std::vector<std::string> popupChoices;

static void DumpItems(const std::vector<std::shared_ptr<RecordingMenuItem>> &items, int depth);

class RecordingMenu final : public Menu {
public:
    std::vector<std::shared_ptr<RecordingMenuItem>> items;

    MenuItemRef AddItem(const std::string &label, std::function<void()> onTrigger,
                        bool mnemonics) override {
        auto it = std::make_shared<RecordingMenuItem>();
        it->label = label;
        it->mnemonics = mnemonics;
        it->onTrigger = onTrigger;
        items.push_back(it);
        return it;
    }
    MenuRef AddSubMenu(const std::string &label) override {
        auto it = std::make_shared<RecordingMenuItem>();
        it->label = label;
        it->subMenu = std::make_shared<RecordingMenu>();
        items.push_back(it);
        return it->subMenu;
    }
    void AddSeparator() override {
        auto it = std::make_shared<RecordingMenuItem>();
        it->separator = true;
        items.push_back(it);
    }
    void PopUp() override {
        printf("popup\n");
        DumpItems(items, 1);
        if(popupChoices.empty()) {
            printf("popup none\n");
            return;
        }
        std::string choice = popupChoices.front();
        popupChoices.erase(popupChoices.begin());
        for(auto &it : items) {
            if(!it->separator && it->Stripped() == choice) {
                printf("popup pick %s\n", choice.c_str());
                if(it->onTrigger) it->onTrigger();
                return;
            }
        }
        printf("popup missing %s\n", choice.c_str());
    }
    void Clear() override { items.clear(); }
};

class RecordingMenuBar final : public MenuBar {
public:
    std::vector<std::shared_ptr<RecordingMenuItem>> menus;

    MenuRef AddSubMenu(const std::string &label) override {
        auto it = std::make_shared<RecordingMenuItem>();
        it->label = label;
        it->subMenu = std::make_shared<RecordingMenu>();
        menus.push_back(it);
        return it->subMenu;
    }
    void Clear() override { menus.clear(); }
};

static void DumpItems(const std::vector<std::shared_ptr<RecordingMenuItem>> &items, int depth) {
    std::string indent(2 * depth, ' ');
    for(auto &it : items) {
        if(it->separator) {
            printf("%s---\n", indent.c_str());
            continue;
        }
        // The Go port has no translations, so no language menu.
        if(it->label == "&Language") continue;
        printf("%s%s", indent.c_str(), it->label.c_str());
        if(!it->accel.empty()) printf(" [%s]", it->accel.c_str());
        printf(" ind=%d en=%d act=%d\n", (int)it->indicator, it->enabled ? 1 : 0,
               it->active ? 1 : 0);
        if(it->subMenu) DumpItems(it->subMenu->items, depth + 1);
    }
}

namespace SolveSpace { namespace Platform {
MenuRef CreateMenu() { return std::make_shared<RecordingMenu>(); }

MenuBarRef GetOrCreateMainMenu(bool *unique) {
    *unique = false;
    return std::make_shared<RecordingMenuBar>();
}
} }

// Windows are 600x600 pixels and record their state.
class RecordingWindow final : public Window {
public:
    Kind        kind;
    bool        visible = false, fullScreen = false, invalidated = false;
    std::string title;
    MenuBarRef  menuBar;
    Cursor      cursor = Cursor::POINTER;
    std::string tooltip;
    bool        editorVisible = false;
    std::string editorText;
    double      scrollPos = 0;

    double GetPixelDensity() override { return 96; }
    double GetDevicePixelRatio() override { return 1; }
    bool IsVisible() override { return visible; }
    void SetVisible(bool v) override { visible = v; }
    void Focus() override {}
    bool IsFullScreen() override { return fullScreen; }
    void SetFullScreen(bool f) override { fullScreen = f; }
    void SetTitle(const std::string &t) override { title = t; }
    void SetMenuBar(MenuBarRef mb) override { menuBar = mb; }
    void GetContentSize(double *width, double *height) override { *width = 600; *height = 600; }
    void SetMinContentSize(double width, double height) override {}
    void FreezePosition(SettingsRef settings, const std::string &key) override {}
    void ThawPosition(SettingsRef settings, const std::string &key) override {}
    void SetCursor(Cursor c) override { cursor = c; }
    void SetTooltip(const std::string &text, double x, double y, double w, double h) override {
        tooltip = text;
    }
    bool IsEditorVisible() override { return editorVisible; }
    void ShowEditor(double x, double y, double fontHeight, double minWidth, bool isMonospace,
                    const std::string &text) override {
        editorVisible = true;
        editorText = text;
    }
    void HideEditor() override { editorVisible = false; }
    void SetScrollbarVisible(bool v) override {}
    void ConfigureScrollbar(double min, double max, double pageSize) override {}
    double GetScrollbarPosition() override { return scrollPos; }
    void SetScrollbarPosition(double pos) override { scrollPos = pos; }
    void Invalidate() override { invalidated = true; }
};

static std::vector<std::shared_ptr<RecordingWindow>> windows;

namespace SolveSpace { namespace Platform {
WindowRef CreateWindow(Window::Kind kind, WindowRef parentWindow) {
    auto w = std::make_shared<RecordingWindow>();
    w->kind = kind;
    windows.push_back(w);
    return w;
}
} }

static RecordingWindow *GW() { return (RecordingWindow *)SS.GW.window.get(); }
static RecordingWindow *TW() { return (RecordingWindow *)SS.TW.window.get(); }

// Dialogs are answered from the script.
static std::vector<std::string> answers;
static std::vector<std::string> files;

class RecordingMessageDialog final : public MessageDialog {
public:
    Type        type = Type::INFORMATION;
    std::string message, description;

    void SetType(Type t) override { type = t; }
    void SetTitle(std::string t) override {}
    void SetMessage(std::string m) override { message = m; }
    void SetDescription(std::string d) override { description = d; }
    void AddButton(std::string label, Response r, bool isDefault) override {}
    Response RunModal() override {
        printf("dialog %d %s|%s\n", (int)type, Esc(message).c_str(), Esc(description).c_str());
        if(answers.empty()) {
            printf("dialog unanswered\n");
            fflush(stdout);
            exit(3);
        }
        std::string a = answers.front();
        answers.erase(answers.begin());
        if(a == "ok") return Response::OK;
        if(a == "yes") return Response::YES;
        if(a == "no") return Response::NO;
        return Response::CANCEL;
    }
};

class RecordingFileDialog final : public FileDialog {
public:
    bool           save = false;
    Platform::Path filename;

    void SetTitle(std::string t) override {}
    void SetCurrentName(std::string name) override { filename = Platform::Path::From(name); }
    Platform::Path GetFilename() override { return filename; }
    void SetFilename(Platform::Path p) override { filename = p; }
    void SuggestFilename(Platform::Path p) override { filename = p; }
    void AddFilter(std::string name, std::vector<std::string> extensions) override {}
    void FreezeChoices(SettingsRef settings, const std::string &key) override {}
    void ThawChoices(SettingsRef settings, const std::string &key) override {}
    bool RunModal() override {
        printf("filedialog %s\n", save ? "save" : "open");
        if(files.empty()) {
            printf("filedialog unanswered\n");
            fflush(stdout);
            exit(3);
        }
        std::string f = files.front();
        files.erase(files.begin());
        if(f == "-") return false;
        filename = Platform::Path::From(f);
        return true;
    }
};

namespace SolveSpace { namespace Platform {
MessageDialogRef CreateMessageDialog(WindowRef parentWindow) {
    return std::make_shared<RecordingMessageDialog>();
}
FileDialogRef CreateOpenFileDialog(WindowRef parentWindow) {
    return std::make_shared<RecordingFileDialog>();
}
FileDialogRef CreateSaveFileDialog(WindowRef parentWindow) {
    auto d = std::make_shared<RecordingFileDialog>();
    d->save = true;
    return d;
}
} }

// The headless build's Cairo canvas needs a surface for each frame, which
// Paint's closing Clear() destroys; a GL canvas has one from its window.
static void PrepareCanvas(std::shared_ptr<ViewportCanvas> canvas) {
    auto c = std::static_pointer_cast<CairoPixmapRenderer>(canvas);
    if(!c) return;
    Camera camera = c->GetCamera();
    camera.width = 600;
    camera.height = 600;
    camera.pixelRatio = 1;
    c->SetCamera(camera);
    c->Init();
}

// The event loop's idle work: timers, then repaints, until quiet.
static void Settle() {
    for(int i = 0; i < 100; i++) {
        bool busy = RunTimers();
        if(GW() && GW()->invalidated) {
            GW()->invalidated = false;
            busy = true;
            PrepareCanvas(SS.GW.canvas);
            if(GW()->onRender) GW()->onRender();
        }
        if(TW() && TW()->invalidated) {
            TW()->invalidated = false;
            busy = true;
            PrepareCanvas(SS.TW.canvas);
            if(TW()->onRender) TW()->onRender();
        }
        if(!busy) return;
    }
}

// Text window rows, as scripts/cpp-textwin-harness.cpp prints them.
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

static void DumpHover() {
    auto &h = SS.GW.hover;
    printf("hover %x %x %d\n", h.entity.v, h.constraint.v, h.emphasized ? 1 : 0);
}

static void Dump() {
    GraphicsWindow &gw = SS.GW;
    printf("pending %d %d \"%s\" %x %x %d %d\n", (int)gw.pending.operation,
           (int)gw.pending.command, gw.pending.description ? gw.pending.description : "",
           gw.pending.request.v, gw.pending.point.v, gw.pending.hasSuggestion ? 1 : 0,
           (int)gw.pending.suggestion);
    DumpHover();
    printf("selection %d", gw.selection.n);
    for(auto &s : gw.selection) printf(" %x/%x", s.entity.v, s.constraint.v);
    printf("\n");
    printf("active %x %x\n", gw.activeGroup.v, gw.ActiveWorkplane().v);
    printf("camera %s %s %s %s\n", G(gw.scale).c_str(), V(gw.offset).c_str(),
           V(gw.projRight).c_str(), V(gw.projUp).c_str());
    printf("counts %d %d %d %d %d\n", SK.groupOrder.n, SK.request.n, SK.entity.n,
           SK.constraint.n, SK.param.n);
    printf("editor %d \"%s\" %d \"%s\"\n", GW()->editorVisible ? 1 : 0,
           Esc(GW()->editorText).c_str(), TW()->editorVisible ? 1 : 0,
           Esc(TW()->editorText).c_str());
    printf("tw %d unsaved %d cursor %d tooltip \"%s\"\n", (int)SS.TW.shown.screen,
           SS.unsaved ? 1 : 0, (int)GW()->cursor, Esc(GW()->tooltip).c_str());
}

static MouseEvent::Button ParseButton(const std::string &s) {
    if(s == "L") return MouseEvent::Button::LEFT;
    if(s == "M") return MouseEvent::Button::MIDDLE;
    if(s == "R") return MouseEvent::Button::RIGHT;
    return MouseEvent::Button::NONE;
}

static void Modifiers(const std::vector<std::string> &t, size_t from, bool *shift, bool *ctrl) {
    for(size_t i = from; i < t.size(); i++) {
        if(t[i] == "shift") *shift = true;
        if(t[i] == "ctrl") *ctrl = true;
    }
}

static void Mouse(RecordingWindow *w, const std::vector<std::string> &t) {
    MouseEvent ev = {};
    std::string op = t[0];
    if(op.compare(0, 2, "tw") == 0) op = op.substr(2);
    size_t mods;
    if(op == "move") {
        ev.type = MouseEvent::Type::MOTION;
        ev.x = std::stod(t[1]);
        ev.y = std::stod(t[2]);
        if(t.size() > 3) ev.button = ParseButton(t[3]);
        mods = 3;
    } else if(op == "scroll") {
        ev.type = MouseEvent::Type::SCROLL_VERT;
        ev.x = std::stod(t[1]);
        ev.y = std::stod(t[2]);
        ev.scrollDelta = std::stod(t[3]);
        mods = 4;
    } else if(op == "leave") {
        ev.type = MouseEvent::Type::LEAVE;
        mods = 1;
    } else {
        ev.type = op == "down" ? MouseEvent::Type::PRESS :
                  op == "up"   ? MouseEvent::Type::RELEASE : MouseEvent::Type::DBL_PRESS;
        ev.button = ParseButton(t[1]);
        ev.x = std::stod(t[2]);
        ev.y = std::stod(t[3]);
        mods = 4;
    }
    Modifiers(t, mods, &ev.shiftDown, &ev.controlDown);
    if(w->onMouseEvent) w->onMouseEvent(ev);
}

static void Key(const std::vector<std::string> &t) {
    KeyboardEvent ev = {};
    ev.key = KeyboardEvent::Key::CHARACTER;
    const std::string &k = t[1];
    if(k == "esc") ev.chr = '\x1b';
    else if(k == "del") ev.chr = '\x7f';
    else if(k == "tab") ev.chr = '\t';
    else if(k == "bs") ev.chr = '\b';
    else if(k == "ret") ev.chr = '\r';
    else if(k == "space") ev.chr = ' ';
    else if(k.size() > 1 && k[0] == 'F') {
        ev.key = KeyboardEvent::Key::FUNCTION;
        ev.num = std::stoi(k.substr(1));
    } else ev.chr = (unsigned char)k[0];
    Modifiers(t, 2, &ev.shiftDown, &ev.controlDown);
    if(!GW()->onKeyboardEvent) return;
    ev.type = KeyboardEvent::Type::PRESS;
    GW()->onKeyboardEvent(ev);
    ev.type = KeyboardEvent::Type::RELEASE;
    GW()->onKeyboardEvent(ev);
}

static std::vector<std::string> Split(const std::string &s, char sep) {
    std::vector<std::string> r;
    std::string cur;
    for(char c : s) {
        if(c == sep) { r.push_back(cur); cur.clear(); } else cur += c;
    }
    r.push_back(cur);
    return r;
}

static void TriggerMenu(const std::string &path) {
    auto bar = std::static_pointer_cast<RecordingMenuBar>(GW()->menuBar);
    const std::vector<std::shared_ptr<RecordingMenuItem>> *items = &bar->menus;
    std::shared_ptr<RecordingMenuItem> found;
    for(const std::string &label : Split(path, '|')) {
        found = nullptr;
        if(!items) break;
        for(auto &it : *items) {
            if(!it->separator && it->Stripped() == label) { found = it; break; }
        }
        if(!found) break;
        items = found->subMenu ? &found->subMenu->items : nullptr;
    }
    if(!found) {
        printf("menu missing %s\n", path.c_str());
        return;
    }
    if(found->onTrigger) found->onTrigger();
}

static void SetView(const std::string &name) {
    GraphicsWindow &gw = SS.GW;
    if(name == "default") {
        gw.scale     = 5;
        gw.offset    = {0, 0, 0};
        gw.projRight = {1, 0, 0};
        gw.projUp    = {0, 1, 0};
    } else {
        gw.scale     = 3.25;
        gw.offset    = Vector::From(-4.5, 2.25, 1.5);
        gw.projRight = Vector::From(0.8, -0.5, 0.2);
        gw.projUp    = Vector::From(0.3, 0.6, 0.75);
        gw.NormalizeProjectionVectors();
    }
    gw.Invalidate();
}

int main(int argc, char **argv) {
    if(argc < 3) return 2;
    for(int i = 3; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    std::string savePath = argv[2];

    SS.Init();
    // Animations jump to their end, so they don't depend on the clock.
    SS.animationSpeed = 0;
    Settle();

    std::ifstream script(argv[1]);
    std::string line;
    while(std::getline(script, line)) {
        if(line.empty() || line[0] == '#') continue;
        std::istringstream ss(line);
        std::vector<std::string> t;
        std::string tok;
        while(ss >> tok) t.push_back(tok);
        if(t.empty()) continue;
        std::string rest = line.size() > t[0].size() + 1 ? line.substr(t[0].size() + 1) : "";
        const std::string &op = t[0];

        if(op == "load") {
            SS.Load(Platform::Path::From(t[1]));
        } else if(op == "view") {
            SetView(t[1]);
        } else if(op == "move" || op == "down" || op == "up" || op == "dbl" ||
                  op == "scroll" || op == "leave") {
            Mouse(GW(), t);
        } else if(op.compare(0, 2, "tw") == 0 && op != "tw") {
            Mouse(TW(), t);
        } else if(op == "key") {
            Key(t);
        } else if(op == "menu") {
            TriggerMenu(rest);
        } else if(op == "popup") {
            popupChoices.push_back(rest);
        } else if(op == "answer") {
            answers.push_back(t[1]);
        } else if(op == "file") {
            files.push_back(t[1]);
        } else if(op == "edit") {
            RecordingWindow *w = GW()->editorVisible ? GW() : TW()->editorVisible ? TW() : NULL;
            if(!w) {
                printf("edit no editor\n");
            } else if(w->onEditingDone) {
                w->onEditingDone(rest);
            }
        } else if(op == "dump") {
            Dump();
        } else if(op == "hover") {
            DumpHover();
            printf("hoverlist %d\n", SS.GW.hoverList.n);
            for(auto &h : SS.GW.hoverList) {
                printf("  %d %s %s %x %x\n", h.zIndex, G(h.distance).c_str(),
                       G(h.depth).c_str(), h.selection.entity.v, h.selection.constraint.v);
            }
        } else if(op == "hovergrid") {
            int n = std::stoi(t[1]);
            for(int i = 0; i < n; i++) {
                for(int j = 0; j < n; j++) {
                    MouseEvent ev = {};
                    ev.type = MouseEvent::Type::MOTION;
                    ev.x = 600.0 * (j + 0.5) / n;
                    ev.y = 600.0 * (i + 0.5) / n;
                    GW()->onMouseEvent(ev);
                    auto &h = SS.GW.hover;
                    printf("at %s %s %x %x %d\n", G(ev.x).c_str(), G(ev.y).c_str(),
                           h.entity.v, h.constraint.v, h.emphasized ? 1 : 0);
                }
            }
        } else if(op == "params") {
            for(auto &p : SK.param) {
                printf("param %x %s %d %d\n", p.h.v, G(p.val).c_str(), p.free ? 1 : 0,
                       p.known ? 1 : 0);
            }
            for(hGroup hg : SK.groupOrder) {
                Group *g = SK.GetGroup(hg);
                printf("group %x %d %d %d %d\n", hg.v, (int)g->solved.how, g->solved.dof,
                       g->dofCheckOk ? 1 : 0, g->clean ? 1 : 0);
            }
        } else if(op == "menus") {
            auto bar = std::static_pointer_cast<RecordingMenuBar>(GW()->menuBar);
            DumpItems(bar->menus, 0);
        } else if(op == "tw") {
            DumpRows();
        } else if(op == "save") {
            if(!SS.SaveToFile(Platform::Path::From(savePath))) {
                printf("save failed\n");
            } else {
                std::ifstream f(savePath, std::ios::binary);
                std::stringstream buf;
                buf << f.rdbuf();
                printf("save\n%send save\n", buf.str().c_str());
            }
        } else {
            fprintf(stderr, "unknown script line: %s\n", line.c_str());
            return 2;
        }
        Settle();
    }
    fflush(stdout);
    _Exit(0);
}
