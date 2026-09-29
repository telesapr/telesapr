// Opens a .slvs file as C++ SolveSpace does (LoadFromFile, then
// AfterNewFile), pins the view, and exports it in every format into a
// folder: the 2d view as each vector format, the wireframe, the section
// through the active workplane, the mesh and the surfaces. The
// headless build has no window, so the camera is GetCamera's fallback,
// 297x210 pixels. pkg/ui/export_test.go runs the same exports, in the
// same order. Any further arguments are font files for TrueType text.
// See scripts/regen-export-golden.sh.
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

    SS.GW.scale     = 5;
    SS.GW.offset    = {0, 0, 0};
    SS.GW.projRight = Vector::From(0.8, -0.5, 0.2);
    SS.GW.projUp    = Vector::From(0.3, 0.6, 0.75);
    SS.GW.NormalizeProjectionVectors();

    Platform::Path dir = Platform::Path::From(argv[2]);
    for(const char *ext : { "svg", "eps", "pdf", "plt", "txt", "step", "dxf" }) {
        SS.ExportViewOrWireframeTo(dir.Join(std::string("view.") + ext), /*exportWireframe=*/false);
    }
    for(const char *ext : { "dxf", "step" }) {
        SS.ExportViewOrWireframeTo(dir.Join(std::string("wireframe.") + ext), /*exportWireframe=*/true);
    }
    // Straight on, so circles stay circles, with diameters shown as
    // radii.
    SS.GW.projRight = Vector::From(1, 0, 0);
    SS.GW.projUp    = Vector::From(0, 1, 0);
    for(Constraint &c : SK.constraint) {
        if(c.type == Constraint::Type::DIAMETER) c.other = true;
    }
    for(const char *ext : { "svg", "eps", "pdf", "dxf" }) {
        SS.ExportViewOrWireframeTo(dir.Join(std::string("front.") + ext), /*exportWireframe=*/false);
    }
    SS.ExportSectionTo(dir.Join("section.svg"));
    // With the export options that are off by default.
    SS.exportShadedTriangles = true;
    SS.exportBackgroundColor = true;
    SS.exportCanvasSizeAuto  = false;
    for(const char *ext : { "svg", "eps", "pdf" }) {
        SS.ExportViewOrWireframeTo(dir.Join(std::string("options.") + ext), /*exportWireframe=*/false);
    }
    SS.exportOffset = 0.5;
    SS.ExportViewOrWireframeTo(dir.Join("offset.txt"), /*exportWireframe=*/false);
    SS.exportOffset = 0;
    for(const char *ext : { "stl", "obj", "js", "wrl" }) {
        SS.ExportMeshTo(dir.Join(std::string("mesh.") + ext));
    }
    StepFileWriter sfw = {};
    sfw.ExportSurfacesTo(dir.Join("surfaces.step"));
    return 0;
}
