// Prints what C++ resource.cpp makes of the bundled fonts: the vector
// font's metrics, widths and traced strokes for some strings, and the
// bitmap font's widths, texture locations and glyph pixels. See
// scripts/regen-res-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

static const char *strings[] = {
    "Hello, world!", "0.123 mm", "\xe2\x88\xa0 45\xc2\xb0", "\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e",
    "\xce\xa9\xe2\x89\x88\xc6\x92", "(x) [y] {z}", "@#$%^&*_+=|\\/<>?~`", "\xef\xbf\xbe\xf0\x9f\x98\x80",
    "g\xc3\xa9om\xc3\xa9trie", "AVAWAY", "",
};

int main() {
    VectorFont *vf = VectorFont::Builtin();
    printf("vector cap %a asc %a desc %a rsb %a\n", vf->capHeight, vf->ascender,
           vf->descender, vf->rightSideBearing);
    for(const char *s : strings) {
        printf("string %zu width %a height %a\n", strlen(s), vf->GetWidth(3.5, s), vf->GetHeight(3.5));
        vf->Trace(3.5, Vector::From(1, 2, 3), Vector::From(1, 0.5, 0), Vector::From(0, 0.25, 1), s,
                  [](Vector a, Vector b) {
            printf("%a %a %a %a %a %a\n", a.x, a.y, a.z, b.x, b.y, b.z);
        });
    }

    BitmapFont bf = BitmapFont::Create();
    const char32_t cps[] = { 'A', ' ', 'g', 0xE000, 0xE003, 0xE007, 0x00B0, 0x2220, 0x65E5,
                             0x672C, 0x8A9E, 0xFFFE, 0x1F600, 0x0416, 0x05D0, 0x3042 };
    for(char32_t cp : cps) {
        double s0, t0, s1, t1;
        size_t w, h;
        bf.LocateGlyph(cp, &s0, &t0, &s1, &t1, &w, &h);
        printf("glyph %x width %zu loc %a %a %a %a %zu %zu\n", (unsigned)cp, bf.GetWidth(cp),
               s0, t0, s1, t1, w, h);
        size_t x0 = (size_t)(s0 * bf.texture->width), y0 = (size_t)(t0 * bf.texture->height);
        for(size_t y = 0; y < h; y++) {
            for(size_t x = 0; x < w; x++) {
                putchar(bf.texture->data[(y0 + y) * bf.texture->stride + x0 + x] ? '#' : '.');
            }
            putchar('\n');
        }
    }
    return 0;
}
