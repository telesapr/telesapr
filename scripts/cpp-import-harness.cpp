// Imports a DXF file into a new sketch as C++ SolveSpace's File > Import
// does (ImportDxf, then GenerateAll up to the active group), and saves
// the sketch. With "3d" after the output file, the new sketch's group is
// first set free in 3d, so the file's entities aren't checked for 3d.
// pkg/ui/importdxf_test.go does the same. See
// scripts/regen-import-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

class DummyMenuItem : public Platform::MenuItem {
public:
    void SetAccelerator(Platform::KeyboardEvent accel) override {}
    void SetIndicator(Indicator type) override {}
    void SetEnabled(bool enabled) override {}
    void SetActive(bool active) override {}
};

int main(int argc, char **argv) {
    if(argc < 3) return 2;
    SS.Init();
    SS.GW.undoMenuItem = std::make_shared<DummyMenuItem>();
    SS.GW.redoMenuItem = std::make_shared<DummyMenuItem>();
    if(argc > 3 && std::string(argv[3]) == "3d") {
        SS.GW.SetWorkplaneFreeIn3d();
    }
    ImportDxf(Platform::Path::From(argv[1]));
    SS.GenerateAll(SolveSpaceUI::Generate::UNTIL_ACTIVE);
    return SS.SaveToFile(Platform::Path::From(argv[2])) ? 0 : 1;
}
