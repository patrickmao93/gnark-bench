package constraint

type NEWCS struct {
	System
	Instructions  []Instruction
	CallData      []uint32 // huge slice.
	NbConstraints int      // can be != than len(instructions

}

type Instruction struct {
	BlueprintID   BlueprintID
	_             uint32 // future use?
	StartCallData uint64 // call data slice (end can be returned by Blueprint)
}

func (cs *NEWCS) AddSparseR1C(c SparseR1C, bID BlueprintID, debugInfo ...DebugInfo) int {
	instruction := cs.compressSparseR1C(&c, bID)
	cs.Instructions = append(cs.Instructions, instruction)
	cs.NbConstraints += 1 // should be 1 here

	return cs.NbConstraints - 1
}

func (cs *NEWCS) compressSparseR1C(c *SparseR1C, bID BlueprintID) Instruction {
	inst := Instruction{
		StartCallData: uint64(len(cs.CallData)),
		BlueprintID:   bID,
	}
	blueprint := cs.Blueprints[bID]
	calldata := blueprint.(BlueprintSparseR1C).CompressSparseR1C(c)
	cs.CallData = append(cs.CallData, calldata...)
	return inst
}

// GetNbConstraints returns the number of constraints
func (cs *NEWCS) GetNbConstraints() int {
	return cs.NbConstraints
}

func (cs *NEWCS) CheckUnconstrainedWires() error {
	// TODO @gbotrel
	return nil
}
