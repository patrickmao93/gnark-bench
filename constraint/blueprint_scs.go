package constraint

type BlueprintGenericSparseR1C struct{}

func (b *BlueprintGenericSparseR1C) NbInputs() int {
	return 10 // xa, xb, qL, qR
}
func (b *BlueprintGenericSparseR1C) NbConstraints() int {
	return 1
}

func (b *BlueprintGenericSparseR1C) CompressSparseR1C(c *SparseR1C) []uint32 {
	return []uint32{
		// generic plonk constraint, the wires first
		c.XA,
		c.XB,
		c.XC,
		c.QL,
		c.QR,
		c.QO,
		c.QM,
		c.QC,
		uint32(c.Commitment),
	}
}

func (b *BlueprintGenericSparseR1C) DecompressSparseR1C(c *SparseR1C, calldata []uint32) {
	c.Clear()
	// TODO @gbotrel use unsafe ptr;
	// calldata := cs.CallData[instruction.StartCallData : instruction.StartCallData+uint32(b.NbInputs())]
	c.XA = calldata[0]
	c.XB = calldata[1]
	c.XC = calldata[2]
	c.QL = calldata[3]
	c.QR = calldata[4]
	c.QO = calldata[5]
	c.QM = calldata[6]
	c.QC = calldata[7]
	c.Commitment = CommitmentConstraint(calldata[8])
}

type BlueprintSparseR1CMul struct{}

func (b *BlueprintSparseR1CMul) NbInputs() int {
	return 4
}
func (b *BlueprintSparseR1CMul) NbConstraints() int {
	return 1
}

func (b *BlueprintSparseR1CMul) CompressSparseR1C(c *SparseR1C) []uint32 {
	return []uint32{
		c.XA,
		c.XB,
		c.XC,
		c.QM,
	}
}

func (b *BlueprintSparseR1CMul) Solve(s Solver, calldata []uint32) {
	m0 := s.GetValue(calldata[3], calldata[0])
	m1 := s.GetValue(CoeffIdOne, calldata[1])
	// qO := s.GetCoeff(calldata[3])

	m0 = s.Mul(m0, m1)
	// m0.Div(qO)

	s.SetValue(calldata[2], m0)

}

func (b *BlueprintSparseR1CMul) DecompressSparseR1C(c *SparseR1C, calldata []uint32) {
	c.Clear()
	c.XA = calldata[0]
	c.XB = calldata[1]
	c.XC = calldata[2]
	c.QO = CoeffIdMinusOne
	c.QM = calldata[3]
}
