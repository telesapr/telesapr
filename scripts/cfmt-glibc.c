// Prints the cases pkg/internal/cfmt/cfmt_test.go compares with glibc's printf:
//     cc -o /tmp/pf scripts/cfmt-glibc.c -lm && /tmp/pf > pkg/internal/cfmt/testdata/glibc.txt
#include <stdio.h>
#include <math.h>
int main(void){
  double vals[] = {0, -0.0, 1, -1, 0.5, 1e-5, 123456789, 1.0/3, 2.5, 3.5, -2.675, 1e21, 1e-300, NAN, -NAN, INFINITY, -INFINITY, 0.000123456, 99999.95, 12.3456789};
  const char *fmts[] = {"%f","%.3f","%.10f","%g","%.6g","%e","%.2e","%10.3f","%-10.3f|","%+.2f","% .2f","%.0f","%#.3g","%lg"};
  for(unsigned i=0;i<sizeof vals/sizeof *vals;i++) for(unsigned j=0;j<sizeof fmts/sizeof *fmts;j++){ printf(fmts[j], vals[i]); printf("\n"); }
  int ivals[] = {0, 5, -5, 255, 123456};
  const char *ifmts[] = {"%d","%5d","%-5d|","%05d","%x","%08x","%02x","%X","%u","%010d"};
  for(unsigned i=0;i<5;i++) for(unsigned j=0;j<10;j++){ printf(ifmts[j], ivals[i]); printf("\n"); }
  printf("%c|%5s|%-5s|%.2s|%%\n", 65, "ab", "ab", "abcdef");
  return 0;
}
