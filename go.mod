module github.com/spacesolve/spacesolve

go 1.27.0

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/msorc/takigo v0.0.0
)

// takigo isn't published yet; it is checked out next to this repository.
replace github.com/msorc/takigo => ../takigo
