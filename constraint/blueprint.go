package constraint

type Blueprint interface {
	NbInputs() int
	NbConstraints() int
}

type BlueprintSolvable interface {
	Solve(s Solver, calldata []uint32)
}

type BlueprintR1C interface {
	CompressR1C(c *R1C) []uint32
	DecompressR1C(into *R1C, calldata []uint32)
}

type BlueprintSparseR1C interface {
	CompressSparseR1C(c *SparseR1C) []uint32
	DecompressSparseR1C(into *SparseR1C, calldata []uint32)
}

type BlueprintSparseR1CBlock interface {
	CompressBlock()
	DecompressBlock() []SparseR1C
}

type BlueprintHint interface {
	CompressHint(HintMapping) []uint32
	DecompressHint(h *HintMapping, calldata []uint32)
}
