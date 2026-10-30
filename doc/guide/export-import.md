# Export and import

Examples: [`examples/scripts/meshquality`](../../examples/scripts/meshquality/meshquality.go),
a cylinder exported as a fine, strictly checked STL;
[`dxfsection`](../../examples/scripts/dxfsection/dxfsection.go), a plate imported from a DXF
file, extruded, and its section written to SVG; [`gltf`](../../examples/scripts/gltf) for glTF;
[`exportscript`](../../examples/scripts/exportscript) for a model written as a script
([Scripts](scripts.md)).

## Export

File > Export writes:

| Entry | Formats | Script |
|---|---|---|
| Image | `.png` | `File.Render(path, w, h, view)` |
| 2d View | `.pdf`, `.eps`, `.svg`, `.dxf`, `.plt`, `.ngc` | `File.Export` |
| 2d Section | the same | `File.ExportSection` |
| 3d Wireframe | `.step`, `.dxf` | `File.ExportAs(ExportWireframe, …)` |
| Triangle Mesh | `.stl`, `.3mf`, `.glb`, `.obj`, `.html`, `.js`, `.wrl` | `File.Export` |
| Surfaces | `.step` | `File.Export` |
| Script | `.go` | `File.ExportScript` |
| Drawing | as 2d View | see [Drawings](drawings.md) |

`File.Export` picks what to write from the extension. Every export is of the model as it is at the
active group.

### Mesh quality

Triangle Mesh asks for three settings, offered as presets:

| Preset | Chord tolerance | Angular tolerance | Segments per curve |
|---|---|---|---|
| draft | 0.5 mm | none | 32 |
| standard (default) | 0.1 mm | none | 64 |
| fine | 0.02 mm | 10° | 256 |

The angular tolerance bounds the turn between neighbouring facets, so small radii are followed
as closely as large ones. **Strict** refuses to write a mesh with naked edges or
self-intersections, where otherwise it is written with a warning. Facets of no area are left out.

```go
doc.File.SetMeshQuality(api.MeshFine)
o := doc.File.ExportOptions()
o.StrictMesh = true
doc.File.SetExportOptions(o)
doc.File.Export("part.stl")
```

A `.glb` carries each face's colour and appearance, in metres ([Appearance](appearance.md)).

### 2d Section

2d Section writes where a plane cuts the solid: the section view's plane when it is on
([Section view](section-view.md)), else the active workplane. A plane that misses the solid
writes nothing and says so. The scale, page and margins are the export options' (Edit >
Preferences, or `ExportOptions` `Scale`, `Canvas`, `Margin`).

## Import

File > Import takes a DXF file into the active group's workplane, or, when it has entities off
its XY plane, as construction entities in 3d. Entities of kinds it doesn't know are left out and
counted. Edit > Rotate Imported 90° turns a linked part a quarter turn (`AsLink().Rotate90`).
SolveSpace's `.slvs` files are imported as new `.tlspr` models.

```go
ignored, err := doc.File.ImportDXF("plate.dxf")
```

STEP, STL and OBJ files are linked rather than imported: see [STEP links](step-links.md).

## The command line

`telesapr-cli` exports without a window: `thumbnail`, `export-view`, `export-section`,
`export-wireframe`, `export-mesh`, `export-surfaces`, `export-drawing` and `export-script`, and
`convert` for `.slvs` files. `--group` exports the model up to a group; `export-mesh` takes
`--quality draft|standard|fine`, `--angle-tol`, `--max-segments` and `--strict`;
`export-section --plane z=15` cuts square to an axis, or `--plane x,y,z,nx,ny,nz` through a point
with a normal. `telesapr-cli --help` lists them all.

    telesapr-cli export-mesh --quality fine --strict --output part.stl part.tlspr
    telesapr-cli export-section --plane z=5 --output cut.svg part.tlspr

## Not yet

DXF is the only 2d format imported; a STEP file can't be imported as editable geometry.
