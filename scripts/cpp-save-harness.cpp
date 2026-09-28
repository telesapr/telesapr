// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile, which generates and solves it) and saves it again
// (SaveToFile generates once more). Any further arguments are font
// files for TrueType text. See scripts/regen-cpp-saves.sh.
#include "solvespace.h"
using namespace SolveSpace;

namespace SolveSpace { namespace Platform {
    extern std::vector<Platform::Path> fontFiles;
} }

int main(int argc, char **argv) {
    if(argc < 3) return 2;
    for(int i = 3; i < argc; i++) {
        Platform::fontFiles.push_back(Platform::Path::From(argv[i]));
    }
    SS.Init();
    if(!SS.LoadFromFile(Platform::Path::From(argv[1]))) return 1;
    SS.AfterNewFile();
    return SS.SaveToFile(Platform::Path::From(argv[2])) ? 0 : 1;
}
