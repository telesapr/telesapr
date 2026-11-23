# The OpenCASCADE half of scripts/step-exchange.sh, run by DRAWEXE:
#
#     STEP_DIR=dir DRAWEXE -b -f scripts/step-exchange/occ-read.tcl
#
# reads each dir/*.step and prints a line for it: the solids' volume and
# centre by vprops at a relative tolerance of 1e-9, whether checkshape
# finds the shape valid, its numbers of solids and faces, and what
# OpenCASCADE's document reader finds of names, colours and validation
# properties. Not FreeCAD's Shape.Volume, nor vprops without a tolerance:
# both were 0.13% off on a full circle written as one rational B-spline.
pload ALL
foreach f [lsort [glob -nocomplain $env(STEP_DIR)/*.step]] {
  set name [file rootname [file tail $f]]
  if {[catch {stepread $f s *} err]} {
    puts "occ\t$name\tread failed: $err"
    continue
  }
  # A file of several roots comes as s_1, s_2, ...; they make one
  # compound here.
  set parts {}
  foreach v [directory s_*] { lappend parts $v }
  eval compound $parts all
  # Solid by solid, the centres weighted by volume: vprops of the
  # compound of two mirrored blocks put their centre 0.625 off.
  set vol 0
  set mx 0
  set my 0
  set mz 0
  explode all So
  foreach so [directory all_*] {
    set r [vprops $so 1e-9 -full]
    regexp {Mass :\s+(\S+)} $r -> v
    regexp {X =\s+(\S+)} $r -> x
    regexp {Y =\s+(\S+)} $r -> y
    regexp {Z =\s+(\S+)} $r -> z
    set vol [expr {$vol + $v}]
    set mx [expr {$mx + $v * $x}]
    set my [expr {$my + $v * $y}]
    set mz [expr {$mz + $v * $z}]
    unset $so
  }
  set cx [expr {$vol != 0 ? $mx / $vol : 0}]
  set cy [expr {$vol != 0 ? $my / $vol : 0}]
  set cz [expr {$vol != 0 ? $mz / $vol : 0}]
  set chk [checkshape all -short]
  set valid [expr {[string match "*seems to be valid*" $chk] ? "valid" : "invalid"}]
  set nb [nbshapes all]
  regexp {SOLID\s+:\s+(\d+)} $nb -> nsol
  regexp {FACE\s+:\s+(\d+)} $nb -> nface
  ReadStep D $f
  set names {}
  foreach l [XGetTopLevelShapes D] { lappend names [GetName D $l] }
  set stat [XStat D]
  set vp 0
  regexp {Number of Volume Props = (\d+)} $stat -> vp
  set colours [llength [XGetAllColors D]]
  Close D
  foreach v [directory s_*] { unset $v }
  puts "occ\t$name\t$vol\t$cx\t$cy\t$cz\t$valid\t$nsol\t$nface\t$colours\t$vp\t[join $names ,]"
}
exit
