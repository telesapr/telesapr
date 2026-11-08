#!/usr/bin/env bash
# Checks that the agent instructions (AGENTS.md, .claude/rules/*.md,
# .claude/skills/*/SKILL.md) name only what exists: make targets, paths,
# test names, TELESAPR_* variables, and package-qualified identifiers.
# The instructions are read by coding agents that trust them, so a
# stale name there costs more than one in a comment. make checks runs it.
#
# Usage:
#     scripts/agents-check.sh [files...]    # default: the files above
set -u
cd "$(dirname "$0")/.."

if [ $# -gt 0 ]; then
  files=("$@")
else
  files=(AGENTS.md)
  while IFS= read -r f; do files+=("$f"); done < <(find .claude/rules .claude/skills -name '*.md' 2>/dev/null | sort)
fi

log=$(mktemp); trap 'rm -f "$log"' EXIT
report() { echo "agents-check: $1: $2" | tee -a "$log"; }

# Backticked spans of all the files, one per line, with the file name.
spans() { grep -oH '`[^`]*`' "${files[@]}" | sed 's/`//g'; }

# 1. make targets: `make name` must be a target of the Makefile.
targets=$(grep -oE '^[a-zA-Z][a-zA-Z0-9_-]*:' Makefile | tr -d ':' | sort -u)
spans | grep -oE '^[^:]*:make [a-z][a-z0-9-]*' | sed 's/:make / /' | sort -u | while read -r f t; do
  grep -qx "$t" <<<"$targets" || report "$f" "no make target '$t'"
done

# 2. Paths: a span that is a path under the tree must exist. A span with
# a space is a command: its first word is what is checked. A placeholder
# (<name>) makes the fixed prefix a glob; a bare file name must exist
# somewhere in the tree.
spans | sed -E 's/^([^:]*:[^ ]*) .*$/\1/' \
  | grep -E '^[^:]*:(pkg|cmd|doc|scripts|testdata|examples|tools|\.github|\.claude)/|:[A-Za-z0-9_.-]+\.(md|yml|yaml|mod|txt)$' \
  | sed -E 's#/\.\.\.$##; s#/$##' | sort -u | while IFS=: read -r f p; do
  [ -n "$p" ] || continue
  case $p in
    *\<*) compgen -G "${p%%<*}*" >/dev/null || report "$f" "nothing matches '$p'" ;;
    *\**) compgen -G "$p" >/dev/null || report "$f" "nothing matches '$p'" ;;
    */*) [ -e "$p" ] || report "$f" "no such path '$p'" ;;
    *) [ -e "$p" ] || [ -n "$(find . -name "$p" -not -path './.git/*' -print -quit)" ] || report "$f" "no file named '$p'" ;;
  esac
done

# 3. Tests and benchmarks: `TestName` or `BenchmarkName` must be defined.
spans | grep -oE '^[^:]*:(Test|Benchmark|Fuzz)[A-Z][A-Za-z0-9_]*' | sort -u | while IFS=: read -r f t; do
  grep -rqE --include='*_test.go' "^func $t\(" pkg cmd || report "$f" "no test '$t'"
done

# 4. TELESAPR_* variables: every one the code or a script reads is in the
# instructions, and every one the instructions name is read somewhere.
inCode=$( { grep -rhoE 'TELESAPR_[A-Z_]+' --include='*.go' pkg cmd; grep -hoE 'TELESAPR_[A-Z_]+' scripts/*.sh scripts/*.py Makefile; } | sort -u)
inDocs=$(grep -ohE 'TELESAPR_[A-Z_]+' "${files[@]}" | sort -u)
comm -23 <(echo "$inCode") <(echo "$inDocs") | while read -r v; do report "instructions" "variable $v is read by the code but not described"; done
comm -13 <(echo "$inCode") <(echo "$inDocs") | while read -r v; do report "instructions" "variable $v is described but nothing reads it"; done

# 5. Package-qualified identifiers: `pkg.Ident` where pkg is a package of
# the tree must be declared in that package (type, func, var, const, or
# a method, whose receiver is not checked).
pkgs=$(find pkg -type d | sed 's#.*/##' | sort -u)
spans | grep -oE '^[^:]*:[a-z][a-z0-9]*\.[A-Z][A-Za-z0-9]*' | sort -u | while IFS=: read -r f id; do
  p=${id%%.*}; n=${id#*.}
  grep -qx "$p" <<<"$pkgs" || continue
  dirs=$(find pkg -type d -name "$p")
  found=0
  for d in $dirs; do
    grep -rqE --include='*.go' "^(func (\([^)]*\) )?$n[\[(]|type $n[ \[]|(var|const) $n |^\s+$n\s*=|^\s+$n\s+[A-Za-z*\[]|^\s+$n$)" "$d" && { found=1; break; }
  done
  [ $found = 1 ] || report "$f" "no '$n' in package $p"
done

if [ -s "$log" ]; then exit 1; fi
echo "agents-check: ok"
