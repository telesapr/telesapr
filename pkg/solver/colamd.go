package solver

import "math"

// This is the COLAMD of Eigen 3.4
// (Eigen/src/OrderingMethods/Eigen_Colamd.h, after Davis's colamd 2.x),
// which SparseQR's COLAMDOrdering uses to order the columns before
// factoring. The column order decides the order of every floating point
// operation in the factorization, so it is ported as is: the same
// workspace size, garbage collections, degree lists and hash buckets,
// and the same tie-breaking.

const colamdEmpty = -1

// colamdCol is Eigen's ColStructure; the unions are single fields.
type colamdCol struct {
	start, length int
	// thickness if alive, parent if dead.
	shared1 int
	// score if alive, order if dead.
	shared2 int
	// headhash, hash or prev.
	shared3 int
	// degree_next or hash_next.
	shared4 int
}

func (c *colamdCol) isDead() bool          { return c.start < 0 }
func (c *colamdCol) isAlive() bool         { return c.start >= 0 }
func (c *colamdCol) isDeadPrincipal() bool { return c.start == -1 }
func (c *colamdCol) killPrincipal()        { c.start = -1 }
func (c *colamdCol) killNonPrincipal()     { c.start = -2 }

// colamdRow is Eigen's RowStructure.
type colamdRow struct {
	start, length int
	// degree, or p in initRowsCols.
	shared1 int
	// mark, or first_column in garbage collection.
	shared2 int
}

func (r *colamdRow) isDead() bool  { return r.shared2 < 0 }
func (r *colamdRow) isAlive() bool { return r.shared2 >= 0 }
func (r *colamdRow) kill()         { r.shared2 = -1 }

// colamdOrdering is Eigen's COLAMDOrdering: perm[c] is the new position
// of column c of the m x n matrix whose columns hold the row indices
// cols[c] (each sorted).
func colamdOrdering(m, n int, cols [][]int) []int {
	nnz := 0
	for _, c := range cols {
		nnz += len(c)
	}
	// Eigen's Alen is recommended(nnz, m, n); colamd then takes the
	// Col and Row arrays from its end, leaving this much for A.
	colSize := (n + 1) * 6
	rowSize := (m + 1) * 4
	alen := 2*nnz + colSize + rowSize + n + nnz/5
	alen -= colSize + rowSize
	a := make([]int, alen)
	p := make([]int, n+1)
	k := 0
	for c, rows := range cols {
		p[c] = k
		k += copy(a[k:], rows)
	}
	p[n] = k

	col := make([]colamdCol, n+1)
	row := make([]colamdRow, m+1)
	colamdInitRowsCols(m, n, row, col, a, p)
	nCol2, maxDeg := colamdInitScoring(m, n, row, col, a, p)
	colamdFindOrdering(m, n, alen, row, col, a, p, nCol2, maxDeg, 2*nnz)
	colamdOrderChildren(n, col, p)

	perm := make([]int, n)
	for i := range n {
		perm[p[i]] = i
	}
	return perm
}

func colamdInitRowsCols(nRow, nCol int, row []colamdRow, col []colamdCol, a, p []int) {
	for c := range nCol {
		col[c].start = p[c]
		col[c].length = p[c+1] - p[c]
		col[c].shared1 = 1
		col[c].shared2 = 0
		col[c].shared3 = colamdEmpty
		col[c].shared4 = colamdEmpty
	}
	for r := range nRow {
		row[r].length = 0
		row[r].shared2 = -1
	}
	jumbled := false
	for c := range nCol {
		lastRow := -1
		for cp := p[c]; cp < p[c+1]; cp++ {
			r := a[cp]
			if r <= lastRow || row[r].shared2 == c {
				jumbled = true
			}
			if row[r].shared2 != c {
				row[r].length++
			} else {
				col[c].length--
			}
			row[r].shared2 = c
			lastRow = r
		}
	}

	row[0].start = p[nCol]
	row[0].shared1 = row[0].start
	row[0].shared2 = -1
	for r := 1; r < nRow; r++ {
		row[r].start = row[r-1].start + row[r-1].length
		row[r].shared1 = row[r].start
		row[r].shared2 = -1
	}

	if jumbled {
		for c := range nCol {
			for cp := p[c]; cp < p[c+1]; cp++ {
				r := a[cp]
				if row[r].shared2 != c {
					a[row[r].shared1] = c
					row[r].shared1++
					row[r].shared2 = c
				}
			}
		}
	} else {
		for c := range nCol {
			for cp := p[c]; cp < p[c+1]; cp++ {
				r := a[cp]
				a[row[r].shared1] = c
				row[r].shared1++
			}
		}
	}

	for r := range nRow {
		row[r].shared2 = 0
		row[r].shared1 = row[r].length
	}

	if jumbled {
		col[0].start = 0
		p[0] = col[0].start
		for c := 1; c < nCol; c++ {
			col[c].start = col[c-1].start + col[c-1].length
			p[c] = col[c].start
		}
		for r := range nRow {
			for rp := row[r].start; rp < row[r].start+row[r].length; rp++ {
				a[p[a[rp]]] = r
				p[a[rp]]++
			}
		}
	}
}

// colamdInitScoring returns n_col2 and max_deg.
func colamdInitScoring(nRow, nCol int, row []colamdRow, col []colamdCol, a, head []int) (int, int) {
	// knobs[DenseRow] and knobs[DenseCol] are 0.5.
	denseRowCount := max(0, min(int(0.5*float64(nCol)), nCol))
	denseColCount := max(0, min(int(0.5*float64(nRow)), nRow))
	maxDeg := 0
	nCol2 := nCol

	for c := nCol - 1; c >= 0; c-- {
		if col[c].length == 0 {
			nCol2--
			col[c].shared2 = nCol2
			col[c].killPrincipal()
		}
	}
	for c := nCol - 1; c >= 0; c-- {
		if col[c].isDead() {
			continue
		}
		if col[c].length > denseColCount {
			nCol2--
			col[c].shared2 = nCol2
			for cp := col[c].start; cp < col[c].start+col[c].length; cp++ {
				row[a[cp]].shared1--
			}
			col[c].killPrincipal()
		}
	}
	for r := range nRow {
		deg := row[r].shared1
		if deg > denseRowCount || deg == 0 {
			row[r].kill()
		} else {
			maxDeg = max(maxDeg, deg)
		}
	}
	for c := nCol - 1; c >= 0; c-- {
		if col[c].isDead() {
			continue
		}
		score := 0
		cp := col[c].start
		newCp := cp
		cpEnd := cp + col[c].length
		for cp < cpEnd {
			r := a[cp]
			cp++
			if row[r].isDead() {
				continue
			}
			a[newCp] = r
			newCp++
			score += row[r].shared1 - 1
			score = min(score, nCol)
		}
		colLength := newCp - col[c].start
		if colLength == 0 {
			nCol2--
			col[c].shared2 = nCol2
			col[c].killPrincipal()
		} else {
			col[c].length = colLength
			col[c].shared2 = score
		}
	}

	for c := 0; c <= nCol; c++ {
		head[c] = colamdEmpty
	}
	for c := nCol - 1; c >= 0; c-- {
		if col[c].isAlive() {
			score := col[c].shared2
			next := head[score]
			col[c].shared3 = colamdEmpty
			col[c].shared4 = next
			if next != colamdEmpty {
				col[next].shared3 = c
			}
			head[score] = c
		}
	}
	return nCol2, maxDeg
}

func colamdFindOrdering(nRow, nCol, alen int, row []colamdRow, col []colamdCol, a, head []int, nCol2, maxDeg, pfree int) {
	// max_mark is INT_MAX - n_col, with Eigen's int StorageIndex.
	maxMark := math.MaxInt32 - nCol
	tagMark := colamdClearMark(nRow, row)
	minScore := 0

	for k := 0; k < nCol2; {
		for minScore < nCol && head[minScore] == colamdEmpty {
			minScore++
		}
		pivotCol := head[minScore]
		next := col[pivotCol].shared4
		head[minScore] = next
		if next != colamdEmpty {
			col[next].shared3 = colamdEmpty
		}
		pivotColScore := col[pivotCol].shared2
		col[pivotCol].shared2 = k
		pivotColThickness := col[pivotCol].shared1
		k += pivotColThickness

		neededMemory := min(pivotColScore, nCol-k)
		if pfree+neededMemory >= alen {
			pfree = colamdGarbageCollection(nRow, nCol, row, col, a, pfree)
			tagMark = colamdClearMark(nRow, row)
		}

		pivotRowStart := pfree
		pivotRowDegree := 0
		col[pivotCol].shared1 = -pivotColThickness
		for cp := col[pivotCol].start; cp < col[pivotCol].start+col[pivotCol].length; cp++ {
			r := a[cp]
			if row[r].isDead() {
				continue
			}
			for rp := row[r].start; rp < row[r].start+row[r].length; rp++ {
				c := a[rp]
				colThickness := col[c].shared1
				if colThickness > 0 && col[c].isAlive() {
					col[c].shared1 = -colThickness
					a[pfree] = c
					pfree++
					pivotRowDegree += colThickness
				}
			}
		}
		col[pivotCol].shared1 = pivotColThickness
		maxDeg = max(maxDeg, pivotRowDegree)

		for cp := col[pivotCol].start; cp < col[pivotCol].start+col[pivotCol].length; cp++ {
			row[a[cp]].kill()
		}

		pivotRowLength := pfree - pivotRowStart
		pivotRow := colamdEmpty
		if pivotRowLength > 0 {
			pivotRow = a[col[pivotCol].start]
		}

		// Compute set differences.
		for rp := pivotRowStart; rp < pivotRowStart+pivotRowLength; rp++ {
			c := a[rp]
			colThickness := -col[c].shared1
			col[c].shared1 = colThickness
			curScore := col[c].shared2
			prev := col[c].shared3
			next := col[c].shared4
			if prev == colamdEmpty {
				head[curScore] = next
			} else {
				col[prev].shared4 = next
			}
			if next != colamdEmpty {
				col[next].shared3 = prev
			}
			for cp := col[c].start; cp < col[c].start+col[c].length; cp++ {
				r := a[cp]
				if row[r].isDead() {
					continue
				}
				setDifference := row[r].shared2 - tagMark
				if setDifference < 0 {
					setDifference = row[r].shared1
				}
				setDifference -= colThickness
				if setDifference == 0 {
					row[r].kill()
				} else {
					row[r].shared2 = setDifference + tagMark
				}
			}
		}

		// Add up set differences for each column.
		for rp := pivotRowStart; rp < pivotRowStart+pivotRowLength; rp++ {
			c := a[rp]
			var hash uint32
			curScore := 0
			cp := col[c].start
			newCp := cp
			cpEnd := cp + col[c].length
			for cp < cpEnd {
				r := a[cp]
				cp++
				if row[r].isDead() {
					continue
				}
				a[newCp] = r
				newCp++
				hash += uint32(r)
				curScore += row[r].shared2 - tagMark
				curScore = min(curScore, nCol)
			}
			col[c].length = newCp - col[c].start
			if col[c].length == 0 {
				col[c].killPrincipal()
				pivotRowDegree -= col[c].shared1
				col[c].shared2 = k
				k += col[c].shared1
			} else {
				col[c].shared2 = curScore
				hash %= uint32(nCol + 1)
				headColumn := head[hash]
				var firstCol int
				if headColumn > colamdEmpty {
					firstCol = col[headColumn].shared3
					col[headColumn].shared3 = c
				} else {
					firstCol = -(headColumn + 2)
					head[hash] = -(c + 2)
				}
				col[c].shared4 = firstCol
				col[c].shared3 = int(hash)
			}
		}

		colamdDetectSuperCols(col, a, head, pivotRowStart, pivotRowLength)

		col[pivotCol].killPrincipal()

		tagMark += maxDeg + 1
		if tagMark >= maxMark {
			tagMark = colamdClearMark(nRow, row)
		}

		// Finalize the new pivot row, and column scores.
		newRp := pivotRowStart
		for rp := pivotRowStart; rp < pivotRowStart+pivotRowLength; rp++ {
			c := a[rp]
			if col[c].isDead() {
				continue
			}
			a[newRp] = c
			newRp++
			a[col[c].start+col[c].length] = pivotRow
			col[c].length++
			curScore := col[c].shared2 + pivotRowDegree
			maxScore := nCol - k - col[c].shared1
			curScore -= col[c].shared1
			curScore = min(curScore, maxScore)
			col[c].shared2 = curScore
			next := head[curScore]
			col[c].shared4 = next
			col[c].shared3 = colamdEmpty
			if next != colamdEmpty {
				col[next].shared3 = c
			}
			head[curScore] = c
			minScore = min(minScore, curScore)
		}

		if pivotRowDegree > 0 {
			row[pivotRow].start = pivotRowStart
			row[pivotRow].length = newRp - pivotRowStart
			row[pivotRow].shared1 = pivotRowDegree
			row[pivotRow].shared2 = 0
		}
	}
}

func colamdOrderChildren(nCol int, col []colamdCol, p []int) {
	for i := range nCol {
		if !col[i].isDeadPrincipal() && col[i].shared2 == colamdEmpty {
			parent := i
			for {
				parent = col[parent].shared1
				if col[parent].isDeadPrincipal() {
					break
				}
			}
			c := i
			order := col[parent].shared2
			for {
				col[c].shared2 = order
				order++
				col[c].shared1 = parent
				c = col[c].shared1
				if col[c].shared2 != colamdEmpty {
					break
				}
			}
			col[parent].shared2 = order
		}
	}
	for c := range nCol {
		p[col[c].shared2] = c
	}
}

func colamdDetectSuperCols(col []colamdCol, a, head []int, rowStart, rowLength int) {
	for rp := rowStart; rp < rowStart+rowLength; rp++ {
		c0 := a[rp]
		if col[c0].isDead() {
			continue
		}
		hash := col[c0].shared3
		headColumn := head[hash]
		var firstCol int
		if headColumn > colamdEmpty {
			firstCol = col[headColumn].shared3
		} else {
			firstCol = -(headColumn + 2)
		}
		for superC := firstCol; superC != colamdEmpty; superC = col[superC].shared4 {
			length := col[superC].length
			prevC := superC
			for c := col[superC].shared4; c != colamdEmpty; c = col[c].shared4 {
				if col[c].length != length || col[c].shared2 != col[superC].shared2 {
					prevC = c
					continue
				}
				cp1, cp2 := col[superC].start, col[c].start
				i := 0
				for ; i < length; i++ {
					if a[cp1+i] != a[cp2+i] {
						break
					}
				}
				if i != length {
					prevC = c
					continue
				}
				col[superC].shared1 += col[c].shared1
				col[c].shared1 = superC
				col[c].killNonPrincipal()
				col[c].shared2 = colamdEmpty
				col[prevC].shared4 = col[c].shared4
			}
		}
		if headColumn > colamdEmpty {
			col[headColumn].shared3 = colamdEmpty
		} else {
			head[hash] = colamdEmpty
		}
	}
}

// colamdGarbageCollection compacts the columns and rows in a[0:pfree]
// and returns the new pfree.
func colamdGarbageCollection(nRow, nCol int, row []colamdRow, col []colamdCol, a []int, pfree int) int {
	pdest := 0
	for c := range nCol {
		if col[c].isAlive() {
			psrc := col[c].start
			col[c].start = pdest
			length := col[c].length
			for range length {
				r := a[psrc]
				psrc++
				if row[r].isAlive() {
					a[pdest] = r
					pdest++
				}
			}
			col[c].length = pdest - col[c].start
		}
	}
	for r := range nRow {
		if row[r].isAlive() {
			if row[r].length == 0 {
				row[r].kill()
			} else {
				psrc := row[r].start
				row[r].shared2 = a[psrc]
				a[psrc] = -r - 1
			}
		}
	}
	psrc := pdest
	for psrc < pfree {
		v := a[psrc]
		psrc++
		if v < 0 {
			psrc--
			r := -a[psrc] - 1
			a[psrc] = row[r].shared2
			row[r].start = pdest
			length := row[r].length
			for range length {
				c := a[psrc]
				psrc++
				if col[c].isAlive() {
					a[pdest] = c
					pdest++
				}
			}
			row[r].length = pdest - row[r].start
		}
	}
	return pdest
}

func colamdClearMark(nRow int, row []colamdRow) int {
	for r := range nRow {
		if row[r].isAlive() {
			row[r].shared2 = 0
		}
	}
	return 1
}
