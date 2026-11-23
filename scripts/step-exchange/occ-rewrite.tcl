# Writes each IN_DIR/*.step again as AP242 into OUT_DIR, as OpenCASCADE
# reads and writes it, for scripts/step-exchange.sh's round trip:
#
#     IN_DIR=in OUT_DIR=out DRAWEXE -b -f scripts/step-exchange/occ-rewrite.tcl
pload ALL
# 5 is AP242; newmodel makes stepwrite take it.
param write.step.schema 5
foreach f [lsort [glob -nocomplain $env(IN_DIR)/*.step]] {
  set name [file rootname [file tail $f]]
  if {[catch {stepread $f s *}]} { continue }
  set parts {}
  foreach v [directory s_*] { lappend parts $v }
  eval compound $parts all
  newmodel
  stepwrite a all $env(OUT_DIR)/$name.step
  foreach v [directory s_*] { unset $v }
}
exit
