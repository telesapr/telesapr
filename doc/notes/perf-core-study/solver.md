# Constraint solving: where the time goes and what was changed

The solver track of the performance study of October 2026 (branch
`perf-core-solver`, from devel 50409a4e). Every change here leaves every
model byte-identical: `scripts/perf-fixture-hashes.sh` gives the same 277
lines before and after, and `go test ./pkg/solver ./pkg/core ./pkg/sketch
./pkg/persist/slvs ./pkg/script/api` passes.

## Summary

- **A sketch of a few hundred segments spends its regeneration in the
  sparse QR, not in writing or evaluating the equations.** On a chain of
  400 dimensioned segments (802 unknowns) the factorisation of the Newton
  step takes 29 ms and the square one 73 ms, against 1.5 ms to write the
  Jacobian symbolically and 25 µs to evaluate it.
- **The QR fill is the cost, and it is an artefact of Eigen's SparseQR,
  which the solver reproduces bit for bit.** The factors of a chain hold
  about n²/8 entries where a banded structure would give 2n. The cause is
  the row order: Eigen's symbolic analysis assumes every row i has an
  entry in column i, and the rows are equations in handle order, which
  have nothing to do with the columns' params. Matching the rows to the
  columns first (a greedy O(nnz) pass) cuts the Newton step's factorisation
  of the 400-chain from 28.5 ms to 0.33 ms. It changes the bytes: the
  Householder sums run in another order. See "Not kept".
- **The rank test of a sketch being drawn was quadratic.** With far more
  unknowns than equations every column past the rank is dead, and Eigen
  computes the column elimination tree again, allocating four arrays, for
  each one: 590 times for a sketch of 300 segments with 10 dimensions. The
  rank test now stops at the first column that cannot be live (same rank,
  proved by a randomised test): 4.4 ms → 0.1 ms per factorisation, and the
  solve of such a sketch, which factors twice, about 10× faster.
- **Adding a constraint to a large sketch was the slow interaction.**
  Building the 150-segment zigzag of `BenchmarkRegen150` one constraint at
  a time took 7 s, 25 ms per constraint by the end; the regeneration of the
  finished sketch, which the benchmark times, takes 2.5 ms, nearly all of
  it outside the solver (the equations are all solved by substitution).
- **Kept, all same bytes:** the rank-only factorisation; the symbolic
  analysis (COLAMD and the elimination tree) reused across the
  factorisations of a solve; the unknowns sorted once instead of on every
  add; one allocation for the Jacobian's values, sparse matrices and the
  elimination tree of a dead column.
- **Measured and not kept:** a flat tape in place of `Expr.Eval` (−28% of
  25 µs, nothing against the factorisation); clearing `mark` and `tval`
  only where touched (they are `memclr`s already).

## Method

Benchmarks in `pkg/solver/bench_test.go` build systems without the window:

- `SolveChain/n`: a chain of n segments, horizontal and vertical in turn,
  with freedom left. Every equation is `a − b`, so the substitution pass
  takes them all and the Jacobian is empty: the cost is bookkeeping.
- `SolveClosed/n`: the same with every segment's length and the first
  point fixed, the square system of a dimensioned sketch.
- `SolveDimensioning/k`: 300 segments of which the first k have a length,
  a sketch being dimensioned: wide, nothing substituted.
- `SolveDrag`: the 100-chain solved with its last point dragged, nudged
  by 0.1 mm each time.
- `WriteJacobian`: the symbolic part alone (copy, bind, fold,
  differentiate) of the closed 100-chain.
- `FactorRank`, `FactorWide`, `FactorSquare`, `FactorLeastSquares`: the
  full factorisation of the Jacobian in the shapes the solver gives it;
  `RankWide`, `RankSquare`: the rank alone, as the rank test asks.
- `EvalTree`/`EvalTape`: the Jacobian's expressions evaluated by the
  recursive walk and by a flat tape compiled once.

Comparisons are benchstat over six interleaved runs of the two test
binaries (`benchstat` installed to the scratchpad, not `go.mod`). The
machine was shared with the other tracks of the study, so the confidence
intervals are wide; the deltas reported are well outside them.

### Baseline (devel, one run)

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| SolveChain/20 | 24 964 | 19 808 | 162 |
| SolveChain/100 | 209 365 | 90 944 | 586 |
| SolveChain/400 | 2 100 167 | 363 396 | 2 110 |
| SolveClosed/20 | 298 770 | 353 698 | 4 237 |
| SolveClosed/100 | 2 456 153 | 2 464 039 | 22 907 |
| SolveClosed/400 | 37 241 670 | 19 160 971 | 99 319 |
| SolveDrag | 210 977 | 91 072 | 588 |
| WriteJacobian | 1 550 198 | 2 150 907 | 18 931 |
| FactorRank/100 | 572 397 | 723 173 | 1 437 |
| FactorRank/400 | 8 014 031 | 9 488 222 | 5 637 |
| FactorWide/10 | 5 271 817 | 8 873 220 | 2 497 |
| FactorWide/50 | 5 016 651 | 8 481 254 | 2 737 |
| FactorLeastSquares/100 | 694 719 | 407 040 | 2 557 |
| FactorLeastSquares/400 | 30 069 012 | 5 844 120 | 13 233 |
| FactorSquare/100 | 1 752 380 | 741 603 | 4 423 |
| FactorSquare/400 | 72 982 089 | 10 701 686 | 22 119 |

## Where the time goes

### The sparse QR and its fill

`pkg/solver/qr.go` is Eigen 3.4's SparseQR, left-looking Householder with
COLAMD, kept bit for bit because the goldens were solved with it. On the
closed chain its factors are nearly dense:

| n | A | nnz(A) | nnz(Q) | nnz(R) | time |
|---:|---|---:|---:|---:|---:|
| 100 | 202×202 | 402 | 5 452 | 10 552 | 1.5 ms |
| 400 | 802×802 | 1 602 | 81 802 | 162 202 | 71 ms |
| 100, transposed (Newton step) | 202×100 | 402 | 5 250 | 2 649 | 0.6 ms |
| 400, transposed | 802×400 | 1 602 | 81 000 | 40 599 | 28.5 ms |

The structure of A is two disjoint paths (the x's and the y's), whose
Cholesky factor has two entries per column under any sensible ordering.
The fill comes from Eigen's symbolic analysis: `coletree` starts with
`firstRowElt[i] = i`, the assumption that row i has an entry on the
diagonal, and `factorize` adds a structural zero there when it has not
(`foundDiag`). The rows are equations, in handle order, and the columns
params, in handle order; nothing relates equation i to param i, so the
assumed diagonal couples far-apart rows and the elimination tree becomes a
long chain. The entries that fill in are numerically nonzero (none of the
81 802 of Q is zero), so they cannot be skipped.

A row permutation that matches row k to a column with an entry in it
(greedy, in column order, O(nnz)) restores the structure for the tall
shape the Newton step factors:

| n | shape | as is | rows matched |
|---:|---|---:|---:|
| 100 | 202×100 | 644 µs, nnz(Q) 5 250 | 40 µs, nnz(Q) 200 |
| 400 | 802×400 | 28.5 ms, nnz(Q) 81 000 | 0.33 ms, nnz(Q) 800 |

For the square shape (rank test of a dimensioned sketch, 802×802) the
greedy matching helps little (71 → 57 ms): there the Householder vectors
accumulate along the chain of the elimination tree whichever rows come
first. That case needs an ordering of the rows and columns together, or
another method (below).

### What a solve does

On the closed 100-chain (profile of `SolveClosed/100` after the changes):
the factorisation 25%, writing the Jacobian 20% (`Expr.clone` 11.5%,
`FoldNode` 3.7%, `deepCopyWithParamsAsPointers` 2.8%), allocation and GC
about 45% between them. Each solve writes the Jacobian from scratch: every
equation is deep-copied with its params bound, folded, differentiated
symbolically for each param it uses, and each derivative folded; the
copies are thrown away after the solve.

On a sketch being dimensioned (`SolveDimensioning/50`, after the
changes): `coletree` 18% (computed again at every dead column before the
rank is reached: a column that depends on an earlier one is dead), the
factorisation 24%, GC 27%.

On the 150-segment zigzag of `BenchmarkRegen150` the regeneration itself
spends its 2.5 ms outside the solver: every equation is `a − b`, so after
substitution the system has 152 unknowns and no equations. What is slow is
building it: each constraint added regenerates with a rank test of up to
98 equations against 600 unknowns, 500 dead columns, and a `coletree` for
each. That is the interaction a user feels.

`Expr.Eval` is not a cost: the 1 030 expressions of the closed 100-chain's
Jacobian evaluate in 25 µs, a tape in 18 µs.

## What was changed

Each line is one commit on `perf-core-solver`.

| Change | Effect |
|---|---|
| Benchmarks (`bench_test.go`) and `factorSparseQROrdered` | — |
| The rank test stops once as many columns are live as rows or columns, whichever is fewer, and keeps no R (`sparseQRRank`); randomised test against the full factorisation | `RankWide/10` 4.4 ms → 72 µs; `SolveDimensioning/10` 10.6 → 0.6 ms |
| Unknowns sorted once, lazily, not on every `AddParam` | `SolveChain/400` 2.1 → 0.4 ms; `SolveDrag` 205 → 94 µs (with the above) |
| Symbolic analysis (`qrSymbolic`: COLAMD ordering, elimination tree) kept per pattern, for the rank tests and the Newton steps of a solve; randomised test holds the factors through the cache to those analysed afresh | with the next two: `SolveDimensioning/50` −17%, bytes of a solve −10 to −50% |
| `EvalJacobian` keeps one backing array; `spFromRows` and `transpose` allocate once; `WriteJacobian` reuses its param set | (as above) |
| The elimination tree of a dead column reuses its arrays; the sparse Jacobian is built from the symbolic pattern instead of scanning the dense values | (as above) |
| `EvalTree`/`EvalTape` benchmark, the tape as a prototype in the test | measurement only |

### All changes against devel (benchstat, six interleaved runs)

Time:

```
                          │     sec/op      │    sec/op      vs base               │
SolveChain/20-32              22.78µ ±  13%   19.20µ ±  15%  -15.72% (p=0.002 n=6)
SolveChain/100-32            202.31µ ±  10%   95.35µ ±  12%  -52.87% (p=0.002 n=6)
SolveChain/400-32            2103.1µ ±   6%   395.9µ ±  11%  -81.17% (p=0.002 n=6)
SolveClosed/20-32             276.7µ ±  11%   252.9µ ±  17%        ~ (p=0.180 n=6)
SolveClosed/100-32            2.353m ±  14%   2.217m ±  16%        ~ (p=0.180 n=6)
SolveClosed/400-32            37.30m ±  17%   32.57m ±   8%  -12.68% (p=0.004 n=6)
SolveDimensioning/10-32     10649.3µ ± 111%   598.5µ ±  12%  -94.38% (p=0.002 n=6)
SolveDimensioning/50-32      12.768m ±  86%   2.241m ±   6%  -82.45% (p=0.002 n=6)
SolveDrag-32                 204.69µ ±  30%   93.90µ ±   9%  -54.12% (p=0.002 n=6)
WriteJacobian-32              1.443m ±  39%   1.457m ±  14%        ~ (p=0.937 n=6)
FactorRank/100-32             505.9µ ±  86%   462.1µ ±   3%   -8.65% (p=0.002 n=6)
FactorRank/400-32             7.149m ±  62%   6.546m ±   5%   -8.43% (p=0.026 n=6)
FactorWide/10-32              4.360m ±  43%   3.273m ±   6%  -24.94% (p=0.002 n=6)
FactorWide/50-32              4.514m ±  16%   3.543m ±  88%        ~ (p=0.065 n=6)
RankWide/10-32              4365.62µ ±  14%   72.29µ ± 231%  -98.34% (p=0.002 n=6)
RankWide/50-32               4676.5µ ±  15%   327.6µ ±   9%  -92.99% (p=0.002 n=6)
RankSquare/100-32             1.623m ±  12%   1.378m ±   4%  -15.06% (p=0.002 n=6)
RankSquare/400-32             75.40m ±   9%   69.30m ±   4%   -8.08% (p=0.002 n=6)
FactorLeastSquares/100-32     645.3µ ±  16%   627.0µ ±  11%        ~ (p=0.589 n=6)
FactorSquare/100-32           1.644m ±  12%   1.598m ±   5%        ~ (p=0.132 n=6)
geomean                       2.054m          921.0µ         -55.15%
```

Allocation:

```
                          │      B/op      │     B/op      vs base                 │
SolveChain/20-32              19.34Ki ± 0%   19.34Ki ± 0%        ~ (p=1.000 n=6) ¹
SolveChain/100-32             88.81Ki ± 0%   89.50Ki ± 0%   +0.77% (p=0.002 n=6)
SolveChain/400-32             354.9Ki ± 0%   358.1Ki ± 0%   +0.90% (p=0.002 n=6)
SolveClosed/20-32             345.4Ki ± 0%   327.6Ki ± 0%   -5.17% (p=0.002 n=6)
SolveClosed/100-32            2.350Mi ± 0%   1.970Mi ± 0%  -16.17% (p=0.002 n=6)
SolveClosed/400-32            18.27Mi ± 0%   12.85Mi ± 0%  -29.70% (p=0.002 n=6)
SolveDimensioning/10-32     17946.9Ki ± 0%   805.7Ki ± 0%  -95.51% (p=0.002 n=6)
SolveDimensioning/50-32      18.247Mi ± 0%   1.896Mi ± 0%  -89.61% (p=0.002 n=6)
SolveDrag-32                  88.94Ki ± 0%   89.62Ki ± 0%   +0.77% (p=0.002 n=6)
WriteJacobian-32              2.051Mi ± 0%   2.048Mi ± 0%   -0.15% (p=0.002 n=6)
FactorRank/100-32            706.22Ki ± 0%   93.80Ki ± 0%  -86.72% (p=0.002 n=6)
FactorRank/400-32            9265.8Ki ± 0%   370.9Ki ± 0%  -96.00% (p=0.002 n=6)
FactorWide/10-32             8665.2Ki ± 0%   213.6Ki ± 0%  -97.54% (p=0.002 n=6)
FactorWide/50-32             8282.5Ki ± 0%   223.8Ki ± 0%  -97.30% (p=0.002 n=6)
RankWide/10-32              8665.22Ki ± 0%   93.66Ki ± 0%  -98.92% (p=0.002 n=6)
RankWide/50-32              8282.47Ki ± 0%   97.65Ki ± 0%  -98.82% (p=0.002 n=6)
RankSquare/100-32             724.2Ki ± 0%   279.3Ki ± 0%  -61.44% (p=0.002 n=6)
RankSquare/400-32            10.206Mi ± 0%   3.792Mi ± 0%  -62.84% (p=0.002 n=6)
FactorLeastSquares/100-32     397.5Ki ± 0%   401.8Ki ± 0%   +1.09% (p=0.002 n=6)
FactorSquare/100-32           724.2Ki ± 0%   732.1Ki ± 0%   +1.09% (p=0.002 n=6)
geomean                       1.666Mi        392.9Ki       -76.97%
```

`FactorRank`, `FactorWide`, `FactorSquare` and `FactorLeastSquares` call the
full factorisation, so they show the allocation changes only; the rank
test's own path is `RankWide` and `RankSquare`, and the solves. The wide
confidence intervals (±86%, ±111%) are the shared machine; the medians of
the open chain, the dimensioning, the drag and the rank test moved by 2 to
60 times.

### The symbolic cache and the allocation changes against the first two

The last three commits against the rank-only factorisation and the lazy
sort, the same way:

```
                          │    sec/op    │    sec/op     vs base               │
SolveClosed/20-32           233.6µ ± 24%   234.7µ ± 16%        ~ (p=0.937 n=6)
SolveClosed/100-32          1.958m ± 21%   2.017m ± 25%        ~ (p=0.132 n=6)
SolveClosed/400-32          31.41m ±  7%   30.81m ± 14%        ~ (p=0.394 n=6)
SolveDimensioning/10-32     588.3µ ± 21%   555.0µ ± 62%        ~ (p=0.180 n=6)
SolveDimensioning/50-32     2.558m ±  7%   2.120m ± 15%  -17.13% (p=0.009 n=6)
SolveDrag-32                87.48µ ± 14%   88.70µ ±  6%        ~ (p=0.937 n=6)
RankSquare/100-32           1.364m ±  4%   1.342m ±  9%        ~ (p=0.132 n=6)
RankSquare/400-32           68.92m ±  4%   66.90m ±  4%   -2.93% (p=0.041 n=6)
FactorLeastSquares/100-32   607.3µ ±  7%   607.7µ ± 17%        ~ (p=0.485 n=6)
FactorSquare/100-32         1.551m ±  4%   1.544m ± 32%        ~ (p=0.699 n=6)
geomean                     1.669m         1.625m         -2.63%
```

```
                          │     B/op      │     B/op      vs base               │
SolveClosed/20-32            336.7Ki ± 0%   327.6Ki ± 0%   -2.72% (p=0.002 n=6)
SolveClosed/100-32           2.166Mi ± 0%   1.970Mi ± 0%   -9.05% (p=0.002 n=6)
SolveClosed/400-32           15.36Mi ± 0%   12.85Mi ± 0%  -16.36% (p=0.002 n=6)
SolveDimensioning/10-32     1165.1Ki ± 0%   805.7Ki ± 0%  -30.84% (p=0.002 n=6)
SolveDimensioning/50-32      3.770Mi ± 0%   1.896Mi ± 0%  -49.70% (p=0.002 n=6)
SolveDrag-32                 88.69Ki ± 0%   89.62Ki ± 0%   +1.06% (p=0.002 n=6)
RankSquare/100-32            317.1Ki ± 0%   279.3Ki ± 0%  -11.93% (p=0.002 n=6)
RankSquare/400-32            3.928Mi ± 0%   3.792Mi ± 0%   -3.47% (p=0.002 n=6)
FactorLeastSquares/100-32    397.5Ki ± 0%   401.8Ki ± 0%   +1.09% (p=0.002 n=6)
FactorSquare/100-32          724.2Ki ± 0%   732.1Ki ± 0%   +1.09% (p=0.002 n=6)
geomean                      1.031Mi        909.9Ki       -13.82%
```

They are allocation changes: −10 to −50% of the bytes of a solve, −17%
of the time of a sketch being dimensioned, and within the noise of the
closed chain, whose time is the factorisation's arithmetic.

## Measured and not kept

- **A flat tape for `Expr.Eval`.** 25.4 → 18.3 µs for the Jacobian of the
  closed 100-chain, −28% of nothing: the factorisation is a thousand times
  that. Not worth a second representation of expressions.
- **Clearing `mark` and `tval` only where touched.** Eigen clears both per
  column; the Go loops compile to `memclr` and are under 1% of the
  factorisation. Zeroing only the touched entries of `tval` would also
  have to find the entries the reflections touch outside the pattern.
- **Matching the rows of the Jacobian to its columns.** 86× on the Newton
  step's factorisation of the 400-chain, but the bytes change. Recorded as
  the first recommendation below.

## Recommendations

Ordered by expected gain over cost. "Goldens" means the change moves the
last bits of solved params, so fixtures regenerate differently; the
sketch-level checks (`pkg/check`, the UI goldens) decide case by case.

1. **Row ordering of the Jacobian before the sparse QR.** Goldens; 2 days.
   Match each row to a column (greedy in COLAMD order, or a maximum
   bipartite matching, as in Dulmage–Mendelsohn), and apply the same to
   the rank test. Measured: the Newton step's factorisation of a 400-chain
   28.5 ms → 0.33 ms; the square rank test only 71 → 57 ms, so pair it
   with 2 or 3. Every model with more than a few dozen constraints gets
   faster; small sketches are unchanged.
2. **A rank test that is not a full QR.** Goldens, in dof counts only
   where the threshold decides differently; 3 days. The rank is the
   integer the dof is counted from; the Q and R are thrown away. A sparse
   LU with partial pivoting, or QR on the transposed (tall) matrix with the
   rows matched, finds the same rank far more cheaply on the square shape
   (the transposed 400-chain factors in 0.33 ms with rows matched, against
   71 ms for the square as is).
3. **Keep the written Jacobian between solves of the same group.** Same
   bytes; 1 week. `WriteJacobian` is 20% of a dimensioned solve and most of
   its allocation. Key the copied, bound, folded equations and their
   partials on the group's constraint, entity and param handles and the
   `Known` set; when only values change (a drag, an edited dimension), bind
   the kept `OpParamPtr` trees to the new `System`'s params (they point at
   `*core.Param` copies, so the copies must be kept with the cache, or the
   trees re-pointed). Folding depends on known params' values, so the cache
   is valid only while the same params are known; a fold that produced a
   constant from a known value must be redone when that value changes,
   which is the case for a param of an earlier group that moved. Simplest
   rule: cache per group, invalidated when any param not in the group
   changes value.
4. **Split the system into connected components.** Same bytes if each
   component's equations and params keep their handle order and the
   components are solved in order, since the QR of a block-diagonal matrix
   under COLAMD… is not the same bytes: COLAMD orders the whole matrix and
   the Householder sums span blocks. Goldens; 1 week. A sketch with
   several unconnected profiles solves each alone; with the row matching
   this is where the fill ends.
5. **Incremental solving during a drag.** Goldens; 2 weeks. Keep the
   factorisation between mouse moves while the pattern is unchanged and
   update only the values (the symbolic analysis is kept already); or take
   one Newton step per frame from the previous solution, as the window
   already does with `Dragged`. The gain is bounded by 1 and 3.
6. **Not recommended: another linear algebra library.** gonum has dense
   QR (O(n³), slower than this at n = 800) and no sparse QR; SuiteSparse
   needs cgo. The fill above is a property of the symbolic analysis, not
   of the arithmetic, and the fix is a permutation.

## Reproducing

```sh
go test -run NONE -bench . -benchmem ./pkg/solver
go test -c -o base.test ./pkg/solver     # at the base commit
go test -c -o new.test ./pkg/solver
for i in 1 2 3 4 5 6; do
  ./base.test -test.run NONE -test.bench 'Solve|Rank' -test.benchmem >> base.txt
  ./new.test  -test.run NONE -test.bench 'Solve|Rank' -test.benchmem >> new.txt
done
benchstat base.txt new.txt
./scripts/perf-fixture-hashes.sh > after.txt && diff before.txt after.txt
```
