# Licence: GPL-3.0-or-later

**Status:** Accepted
**Date:** 2026-10-03

## Context

TeleSAPR contains code ported from SolveSpace (GPL-3.0-or-later), from the GNU C Library's libm
(LGPL-2.1-or-later), from Eigen (MPL-2.0), from FreeType (FTL) and from libdxfrw, and bundles
fonts and icons under their own terms. The licence of the whole has to be compatible with each.

## Decision

TeleSAPR is distributed under the GNU General Public License, version 3 or later (`LICENSE`),
the licence of the code it began from. `NOTICE` lists every ported or bundled work with the
notice its source asks for. Version numbers start at 0.1.0.

## Consequences

Contributions are GPL. A dependency must be GPL-3-compatible before it is added; the private
windowing library takigo is used under its own terms by the same author.
