# spacesolve (Go Port)

This is a Go port of spacesolve, a parametric 2D/3D CAD tool.

## Project Structure

```
go-spacesolve/
├── cmd/
│   └── spacesolve/      # Main application entry point
├── pkg/
│   ├── core/            # Core data structures (handles, entities, constraints)
│   ├── sketch/          # Sketch and group management
│   ├── system/          # Constraint solver
│   ├── expr/            # Expression evaluator
│   ├── geom/            # Geometric primitives and algorithms
│   ├── mesh/            # Mesh generation and operations
│   ├── render/          # Rendering abstractions
│   ├── ui/              # User interface
│   ├── file/            # File I/O
│   ├── export/          # Export formats (DXF, STEP, etc.)
│   └── platform/        # Platform-specific code
└── test/                # Test files
```

## Build

```bash
go build ./cmd/spacesolve
```

## Original C++ Source

This port is based on the C++ version at: https://github.com/solvespace/solvespace
