// Prints what C++ SolveSpace's Eigen computes for the least-squares cases in
// testdata/solver/sparseqr/cases.txt; see scripts/regen-sparseqr-golden.sh.
// Reads cases: "m n nnz", nnz lines "i j value", then m lines of B (hex
// floats). Prints, per case, the rank SparseQR finds for A and then X as
// System::SolveLeastSquares computes it (unscaled), or SolveLinearSystem
// with the normal equations when m > n.
#include <Eigen/SparseCore>
#include <Eigen/SparseQR>
#include <Eigen/OrderingMethods>
#include <cstdio>
using namespace Eigen;
static bool SolveLinearSystem(const SparseMatrix<double> &A, const VectorXd &B, VectorXd *X) {
    if(A.outerSize() == 0) return true;
    SparseQR<SparseMatrix<double>, COLAMDOrdering<int>> solver;
    solver.compute(A);
    *X = solver.solve(B);
    return (solver.info() == Success);
}
int main() {
    int m, n, nnz;
    while(scanf("%d %d %d", &m, &n, &nnz) == 3) {
        SparseMatrix<double> A(m, n);
        for(int k = 0; k < nnz; k++) { int i, j; double v; scanf("%d %d %la", &i, &j, &v); A.insert(i, j) = v; }
        A.makeCompressed();
        VectorXd B(m);
        for(int i = 0; i < m; i++) scanf("%la", &B[i]);
        SparseQR<SparseMatrix<double>, COLAMDOrdering<int>> rs;
        rs.compute(A);
        printf("rank %d\n", (int)rs.rank());
        VectorXd X;
        if(m <= n) {
            SparseMatrix<double> At = A.transpose();
            At.makeCompressed();
            SparseQR<SparseMatrix<double>, COLAMDOrdering<int>> solver;
            solver.compute(At);
            const int rank = (int)solver.rank();
            VectorXd rhs = solver.colsPermutation().transpose() * B;
            VectorXd v = VectorXd::Zero(m);
            if(rank > 0) {
                SparseMatrix<double> R = solver.matrixR().topLeftCorner(rank, rank);
                VectorXd w = R.triangularView<Upper>().transpose().solve(rhs.topRows(rank));
                v.topRows(rank) = R.triangularView<Upper>().solve(w);
            }
            VectorXd z = solver.colsPermutation() * v;
            X = A.transpose() * z;
        } else {
            SparseMatrix<double> AAt = A * A.transpose();
            AAt.makeCompressed();
            VectorXd z(m);
            SolveLinearSystem(AAt, B, &z);
            X = A.transpose() * z;
        }
        for(int i = 0; i < n; i++) printf("%a\n", X[i]);
    }
}
