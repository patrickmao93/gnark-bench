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
		c.L.VID,
		c.R.VID,
		c.O.VID,
		// coeffs
		c.L.CID,
		c.R.CID,
		c.O.CID,
		c.M[0].CID,
		c.M[1].CID,
		uint32(c.K),
		uint32(c.Commitment),
	}
}

func (b *BlueprintGenericSparseR1C) DecompressSparseR1C(c *SparseR1C, calldata []uint32) {
	c.Clear()

	// calldata := cs.CallData[instruction.StartCallData : instruction.StartCallData+uint32(b.NbInputs())]

	c.L.VID = calldata[0]
	c.R.VID = calldata[1]
	c.O.VID = calldata[2]
	c.L.CID = calldata[3]
	c.R.CID = calldata[4]
	c.O.CID = calldata[5]
	c.M[0].CID = calldata[6]
	c.M[1].CID = calldata[7]
	c.M[0].VID = c.L.VID
	c.M[1].VID = c.R.VID
	c.K = int(calldata[8])
	c.Commitment = CommitmentConstraint(calldata[9])
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
		c.M[0].CID,
		c.M[0].VID,
		c.M[1].VID,
		c.O.VID,
	}
}

func (b *BlueprintSparseR1CMul) Solve(s Solver, calldata []uint32) {
	m0 := s.GetValue(calldata[0], calldata[1])
	m1 := s.GetValue(CoeffIdOne, calldata[2])
	// qO := s.GetCoeff(calldata[3])

	m0 = s.Mul(m0, m1)
	// m0.Div(qO)

	s.SetValue(calldata[3], m0)

}

func (b *BlueprintSparseR1CMul) DecompressSparseR1C(c *SparseR1C, calldata []uint32) {
	c.Clear()

	c.M[0].CID = calldata[0]
	c.M[0].VID = calldata[1]
	c.M[1].CID = CoeffIdOne
	c.M[1].VID = calldata[2]
	c.O.CID = CoeffIdMinusOne
	c.O.VID = calldata[3]

	c.L.VID = c.M[0].VID
	c.R.VID = c.M[1].VID
}
