// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile) and prints each group's display items as
// Group::GenerateDisplayItems leaves them with the default view: the
// mesh's triangle count and every outline, joined into polylines by
// PolylineBuilder. Any further arguments are font files for TrueType
// text. See scripts/regen-display-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

static void PrintVector(const Vector &v) {
    printf(" %a %a %a", v.x, v.y, v.z);
}

int main(int argc, char **argv) {
    if(argc < 2) return 2;
    for(int i = 2; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    SS.Init();
    if(!SS.LoadFromFile(Platform::Path::From(argv[1]))) return 1;
    SS.AfterNewFile();
    for(hGroup hg : SK.groupOrder) {
        Group *g = SK.GetGroup(hg);
        g->displayDirty = true;
        g->GenerateDisplayItems();
        printf("group %x mesh %d outlines %d\n", hg.v,
               g->displayMesh.l.n, g->displayOutlines.l.n);
        for(const SOutline &so : g->displayOutlines.l) {
            printf("%d", so.tag);
            PrintVector(so.a);
            PrintVector(so.b);
            PrintVector(so.nl);
            PrintVector(so.nr);
            printf("\n");
        }
    }
    return 0;
}
