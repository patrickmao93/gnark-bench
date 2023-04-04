package constraint

type NEWCS struct {
	System
	Instructions  []Instruction
	CallData      []uint32 // huge slice.
	NbConstraints int      // can be != than len(instructions

}

func (cs *NEWCS) GetCallData(instruction Instruction) []uint32 {
	blueprint := cs.Blueprints[instruction.BlueprintID]
	nbInputs := blueprint.NbInputs()
	if nbInputs < 0 {
		// happens with R1C of unknown size
		nbInputs = int(cs.CallData[instruction.StartCallData])
	}
	return cs.CallData[instruction.StartCallData : instruction.StartCallData+uint64(nbInputs)]
}

type Instruction struct {
	BlueprintID   BlueprintID
	_             uint32 // future use?
	StartCallData uint64 // call data slice (end can be returned by Blueprint)
}

func (cs *NEWCS) AddConstraint(c R1C, bID BlueprintID, debugInfo ...DebugInfo) int {
	instruction := cs.compressR1C(&c, bID)
	cs.Instructions = append(cs.Instructions, instruction)
	cs.NbConstraints += 1 // should be 1 here

	return cs.NbConstraints - 1
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

func (cs *NEWCS) compressR1C(c *R1C, bID BlueprintID) Instruction {
	inst := Instruction{
		StartCallData: uint64(len(cs.CallData)),
		BlueprintID:   bID,
	}
	blueprint := cs.Blueprints[bID]
	calldata := blueprint.(BlueprintR1C).CompressR1C(c)
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
