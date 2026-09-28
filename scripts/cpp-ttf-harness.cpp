// Dumps what C++ SolveSpace's ttf.cpp computes, for comparing pkg/ttf
// with it: every font it lists (the files given on the command line,
// then the builtin one), its cap height, and for each string on stdin
// (one per line) the aspect ratio and the Beziers PlotString makes,
// without and with kerning. Numbers are printed with %a. Run it from
// solvespace/build/bin, where the builtin font's resource is. See
// scripts/regen-ttf-golden.sh.
#include "solvespace.h"
#include <iostream>
using namespace SolveSpace;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

static void PrintVector(Vector v) {
    printf(" %a %a %a", v.x, v.y, v.z);
}

int main(int argc, char **argv) {
    for(int i = 1; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    std::vector<std::string> strs;
    std::string line;
    while(std::getline(std::cin, line)) strs.push_back(line);

    SS.fonts.LoadAll();
    // Copy the names out first: LoadFont keeps faces open in the list.
    std::vector<std::string> files;
    for(TtfFont &tf : SS.fonts.l) {
        printf("font %s|%s\n", tf.FontFileBaseName().c_str(), tf.name.c_str());
        files.push_back(tf.FontFileBaseName());
    }

    Vector origin = Vector::From(1.5, -2.25, 3.0),
           u      = Vector::From(0.3, 0.4, 0.1),
           v      = Vector::From(-0.2, 0.7, 0.5);
    for(const std::string &file : files) {
        TtfFont *tf = SS.fonts.LoadFont(file);
        printf("load %s %a\n", file.c_str(), tf ? tf->capHeight : 0.0);
        for(size_t i = 0; i < strs.size(); i++) {
            for(int kerning = 0; kerning < 2; kerning++) {
                double ar = SS.fonts.AspectRatio(file, strs[i], kerning != 0);
                SBezierList sbl = {};
                SS.fonts.PlotString(file, strs[i], &sbl, kerning != 0, origin, u, v);
                printf("str %zu %d %a %d\n", i, kerning, ar, sbl.l.n);
                for(SBezier &sb : sbl.l) {
                    printf("b %d", sb.deg);
                    for(int j = 0; j <= sb.deg; j++) {
                        PrintVector(sb.ctrl[j]);
                        printf(" %a", sb.weight[j]);
                    }
                    printf("\n");
                }
                sbl.Clear();
            }
        }
    }
    return 0;
}
