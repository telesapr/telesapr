// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile, which generates and solves it) and saves it again
// (SaveToFile generates once more). See scripts/regen-cpp-saves.sh.
#include "solvespace.h"
using namespace SolveSpace;
int main(int argc, char **argv) {
    if(argc != 3) return 2;
    SS.Init();
    if(!SS.LoadFromFile(Platform::Path::From(argv[1]))) return 1;
    SS.AfterNewFile();
    return SS.SaveToFile(Platform::Path::From(argv[2])) ? 0 : 1;
}
