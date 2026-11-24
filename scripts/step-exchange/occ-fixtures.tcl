# Writes testdata/step/exchange: shapes OpenCASCADE builds and writes as
# STEP, for TestExchangeFixtures (pkg/persist/step) to read, and
# reference.txt, each shape's volume, area and centre as OpenCASCADE
# measures the shape it built, before writing it (vprops and sprops at
# a relative tolerance of 1e-9). Run from the top of the tree:
#
#     DRAWEXE -b -f scripts/step-exchange/occ-fixtures.tcl
#
# Every shape is written as AP242; two also as AP203 and AP214, whose
# files differ in their product structure and styles. The numbers are
# OpenCASCADE's of its own shapes, not of the files read back: reading
# back its own fused file, it found 27455 mm³ where it had built 31644.
pload ALL
set out testdata/step/exchange
file mkdir $out

box b 0 0 0 40 30 20
pcylinder cyl 10 30
pcone cone 12 4 25
psphere ball 15
ptorus ring 20 5
box bf 0 0 0 40 30 20
explode bf E
blend fillet bf 4 bf_1 4 bf_2 4 bf_3 4 bf_4
box bb 0 0 0 40 40 20
pcylinder h 8 40
ttranslate h 20 20 -10
bcut hole bb h
# A ball fused at a corner of the holed block's top: its lower pole lies
# on the block's upright edge.
psphere s2 10
ttranslate s2 40 40 20
bfuse fused hole s2
# A block with two cavities, a box and a ball, neither reaching its
# sides: a solid of three shells, which OpenCASCADE writes as a
# BREP_WITH_VOIDS.
box hb 0 0 0 60 30 30
box hc 5 10 10 20 10 10
psphere hs 8
ttranslate hs 42 15 15
bcut h1 hb hc
bcut hollow h1 hs

set shapes {b cyl cone ball ring fillet hole fused hollow}
set f [open $out/reference.txt w]
puts $f "# name, volume (mm3), area (mm2), centre x, y, z: separated by tabs."
puts $f "# OpenCASCADE's, by vprops and sprops at 1e-9 of the shapes it built,"
puts $f "# written by scripts/step-exchange/occ-fixtures.tcl."
foreach n $shapes {
  set r [vprops $n 1e-9 -full]
  regexp {Mass :\s+(\S+)} $r -> vol
  regexp {X =\s+(\S+)} $r -> cx
  regexp {Y =\s+(\S+)} $r -> cy
  regexp {Z =\s+(\S+)} $r -> cz
  set a [sprops $n 1e-9 -full]
  regexp {Mass :\s+(\S+)} $a -> area
  puts $f "$n\t$vol\t$area\t$cx\t$cy\t$cz"
}
close $f

# write.step.schema: 3 is AP203, 4 AP214, 5 AP242; newmodel makes
# stepwrite take it.
foreach {schema num} {AP242 5 AP214 4 AP203 3} {
  param write.step.schema $num
  foreach n $shapes {
    if {$schema ne "AP242" && $n ne "hole" && $n ne "fused"} { continue }
    newmodel
    stepwrite a $n $out/${n}_$schema.step
  }
}
exit
