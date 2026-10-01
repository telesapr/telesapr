module github.com/spacesolve/spacesolve

go 1.27.0

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/msorc/takigo v0.0.0
	github.com/traefik/yaegi v0.16.1
)

// takigo isn't published yet; its master branch is checked out next to
// this repository, as a worktree of the takigo repository:
//
//	git -C ../takigo worktree add ../takigo-master master
//
// ../takigo itself may be on a branch whose API has moved on.
replace github.com/msorc/takigo => ../takigo-master

tool github.com/traefik/yaegi/cmd/yaegi
