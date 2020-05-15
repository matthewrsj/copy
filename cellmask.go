package towercontroller

func newCellMask(present []bool) []uint32 {
	cm := make([]uint32, len(present)/32)

	for i, cell := range present {
		if !cell {
			continue
		}

		cm[i/32] |= 1 << (i % 32)
	}

	return cm
}
