// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile), and prints every call GraphicsWindow::Draw makes on a
// canvas, for two views: the default one, and a rotated, zoomed one.
// The headless build has no window, so the camera is GetCamera's
// fallback, 297x210 pixels. Any further arguments are font files for
// TrueType text. See scripts/regen-draw-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

static void P(const Vector &v) { printf(" %a %a %a", v.x, v.y, v.z); }
static void C(RgbaColor c) { printf(" %02x%02x%02x%02x", c.red, c.green, c.blue, c.alpha); }

class RecordingCanvas : public Canvas {
public:
    Camera camera = {};

    const Camera &GetCamera() const override { return camera; }

    void S(hStroke hcs) {
        Stroke *s = strokes.FindById(hcs);
        printf(" | stroke %d %d", (int)s->layer, s->zIndex);
        C(s->color);
        printf(" %a %d %d %a", s->width, (int)s->unit, (int)s->stipplePattern, s->stippleScale);
    }
    void F(hFill hcf) {
        Fill *f = fills.FindById(hcf);
        printf(" | fill %d %d", (int)f->layer, f->zIndex);
        C(f->color);
        printf(" %d %d", (int)f->pattern, f->texture ? 1 : 0);
    }

    void DrawLine(const Vector &a, const Vector &b, hStroke hcs) override {
        printf("line"); P(a); P(b); S(hcs); printf("\n");
    }
    void DrawEdges(const SEdgeList &el, hStroke hcs) override {
        printf("edges %d", el.l.n); S(hcs); printf("\n");
        for(const SEdge &e : el.l) { printf(" e"); P(e.a); P(e.b); printf("\n"); }
    }
    bool DrawBeziers(const SBezierList &bl, hStroke hcs) override { return false; }
    void DrawOutlines(const SOutlineList &ol, hStroke hcs, DrawOutlinesAs drawAs) override {
        printf("outlines %d %d", ol.l.n, (int)drawAs); S(hcs); printf("\n");
        for(const SOutline &o : ol.l) { printf(" o %d", o.tag); P(o.a); P(o.b); printf("\n"); }
    }
    void DrawVectorText(const std::string &text, double height,
                        const Vector &o, const Vector &u, const Vector &v,
                        hStroke hcs) override {
        printf("text [%s] %a", text.c_str(), height); P(o); P(u); P(v); S(hcs); printf("\n");
    }
    void DrawQuad(const Vector &a, const Vector &b, const Vector &c, const Vector &d,
                  hFill hcf) override {
        printf("quad"); P(a); P(b); P(c); P(d); F(hcf); printf("\n");
    }
    void DrawPoint(const Vector &o, hStroke hcs) override {
        printf("point"); P(o); S(hcs); printf("\n");
    }
    void DrawPolygon(const SPolygon &p, hFill hcf) override {
        printf("polygon %d", p.l.n); F(hcf); printf("\n");
        for(const SContour &sc : p.l) {
            printf(" c %d", sc.l.n);
            for(const SPoint &sp : sc.l) P(sp.p);
            printf("\n");
        }
    }
    void DrawMesh(const SMesh &m, hFill hcfFront, hFill hcfBack) override {
        printf("mesh %d", m.l.n); F(hcfFront); if(hcfBack.v) F(hcfBack); printf("\n");
        for(const STriangle &t : m.l) {
            printf(" t %u", t.meta.face); C(t.meta.color); P(t.a); P(t.b); P(t.c); printf("\n");
        }
    }
    void DrawFaces(const SMesh &m, const std::vector<uint32_t> &faces, hFill hcf) override {
        printf("faces %d", (int)faces.size());
        for(uint32_t f : faces) printf(" %x", f);
        F(hcf); printf("\n");
    }
    void DrawPixmap(std::shared_ptr<const Pixmap> pm,
                    const Vector &o, const Vector &u, const Vector &v,
                    const Point2d &ta, const Point2d &tb, hFill hcf) override {
        printf("pixmap %d %d", pm ? (int)pm->width : 0, pm ? (int)pm->height : 0);
        P(o); P(u); P(v); printf(" %a %a %a %a", ta.x, ta.y, tb.x, tb.y); F(hcf); printf("\n");
    }
    void InvalidatePixmap(std::shared_ptr<const Pixmap> pm) override {}
};

static void DrawView(const char *name) {
    printf("view %s\n", name);
    RecordingCanvas canvas;
    canvas.camera = SS.GW.GetCamera();
    SS.GW.Draw(&canvas);
}

int main(int argc, char **argv) {
    if(argc < 2) return 2;
    for(int i = 2; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    SS.Init();
    if(!SS.LoadFromFile(Platform::Path::From(argv[1]))) return 1;
    SS.AfterNewFile();

    SS.GW.scale     = 5;
    SS.GW.offset    = {0, 0, 0};
    SS.GW.projRight = {1, 0, 0};
    SS.GW.projUp    = {0, 1, 0};
    DrawView("default");

    SS.GW.scale     = 3.25;
    SS.GW.offset    = Vector::From(-4.5, 2.25, 1.5);
    SS.GW.projRight = Vector::From(0.8, -0.5, 0.2);
    SS.GW.projUp    = Vector::From(0.3, 0.6, 0.75);
    SS.GW.NormalizeProjectionVectors();
    DrawView("rotated");
    return 0;
}
