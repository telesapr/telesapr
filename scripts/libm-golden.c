/* Prints glibc's sin, cos, tan, asin, acos, atan and atan2 for inputs
   spread over each function's branches, plus special values, as
   "name x [y] result" with the bits in hex. See
   scripts/regen-libm-golden.sh; pkg/libm's TestLibmVsGlibc requires
   the port to give the same bits. */
#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static uint64_t state = 1;
static uint64_t next(void) { /* splitmix64 */
    uint64_t z = (state += 0x9e3779b97f4a7c15ull);
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ull;
    z = (z ^ (z >> 27)) * 0x94d049bb133111ebull;
    return z ^ (z >> 31);
}
static double unif(void) { return (next() >> 11) * 0x1p-53; }
static uint64_t bits(double x) { uint64_t b; memcpy(&b, &x, 8); return b; }

/* x in [lo, hi], log-uniform when the range spans decades, either sign. */
static double sample(double lo, double hi) {
    double x = (lo > 0 && hi / lo > 16) ? exp(log(lo) + unif() * (log(hi) - log(lo)))
                                        : lo + unif() * (hi - lo);
    return (next() & 1) ? -x : x;
}

typedef double (*fn1)(double);
#define N 600

static void one(const char *name, fn1 f, const double *r, int nr, const double *sp, int nsp) {
    for(int i = 0; i + 1 < nr; i += 2)
        for(int k = 0; k < N; k++) {
            double x = sample(r[i], r[i + 1]);
            printf("%s %016llx %016llx\n", name, (unsigned long long)bits(x), (unsigned long long)bits(f(x)));
        }
    for(int i = 0; i < nsp; i++) {
        double v[] = { sp[i], -sp[i], nextafter(sp[i], INFINITY), nextafter(sp[i], -INFINITY) };
        for(int j = 0; j < 4; j++)
            printf("%s %016llx %016llx\n", name, (unsigned long long)bits(v[j]), (unsigned long long)bits(f(v[j])));
    }
}

int main(void) {
    const double sp[] = { 0, INFINITY, NAN, 1, 0.5, 2, M_PI, M_PI_2, M_PI_4, 1e-300, 5e-324, 1.7976931348623157e308,
        0.125, 0.0625, 16, 0.126, 0.855469, 2.426265, 25, 1e8, 105414350, 0.0608, 0.787, 1.259e-8, 0.75,
        0.921875, 0.953125, 0.96875, 2.77556e-17, 1.29e-8, 5.805e15 };
    const int nsp = sizeof sp / sizeof sp[0];
    const double trig[] = { 1e-300, 1e-8, 1e-8, 0.126, 0.126, 0.855469, 0.855469, 2.426265, 2.426265, 30,
        30, 1e8, 1e8, 1e300, 0, 7 };
    const double inv[] = { 1e-300, 0.125, 0.125, 0.5, 0.5, 0.75, 0.75, 0.921875, 0.921875, 0.953125,
        0.953125, 0.96875, 0.96875, 1, 0.999999, 1 };
    const double at[] = { 1e-300, 1e-8, 1e-8, 0.0625, 0.0625, 1, 1, 16, 16, 1e16, 1e16, 1e300 };
    one("sin", sin, trig, 16, sp, nsp);
    one("cos", cos, trig, 16, sp, nsp);
    one("tan", tan, trig, 16, sp, nsp);
    one("asin", asin, inv, 16, sp, nsp);
    one("acos", acos, inv, 16, sp, nsp);
    one("atan", atan, at, 12, sp, nsp);
    /* atan2: each branch (sign of x, which of |x| and |y| is larger,
       the ratio below or above 1/16), then pairs of special values. */
    for(int br = 0; br < 8; br++)
        for(int k = 0; k < N; k++) {
            double a = fabs(sample(1e-3, 1e3));
            double b = a * ((br & 1) ? fabs(sample(1.0 / 16, 1)) : fabs(sample(1e-6, 1.0 / 16)));
            double y = (br & 2) ? a : b, x = (br & 2) ? b : a;
            if(br & 4) x = -x;
            if(next() & 1) y = -y;
            printf("atan2 %016llx %016llx %016llx\n", (unsigned long long)bits(y), (unsigned long long)bits(x),
                   (unsigned long long)bits(atan2(y, x)));
        }
    const double sp2[] = { 0, INFINITY, NAN, 1, 2, 1e-300, 1e300, 5e-324 };
    for(int i = 0; i < 8; i++)
        for(int j = 0; j < 8; j++)
            for(int s = 0; s < 4; s++) {
                double y = (s & 1) ? -sp2[i] : sp2[i], x = (s & 2) ? -sp2[j] : sp2[j];
                printf("atan2 %016llx %016llx %016llx\n", (unsigned long long)bits(y), (unsigned long long)bits(x),
                       (unsigned long long)bits(atan2(y, x)));
            }
    return 0;
}
