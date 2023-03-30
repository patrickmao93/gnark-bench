package constraint

import (
	"github.com/consensys/gnark/debug"
	"github.com/consensys/gnark/profile"
)

type NEWCS struct {
	System
	Instructions  []Instruction
	CallData      []uint32 // huge slice.
	NbConstraints int      // can be != than len(instructions

	// we may want to store blueprints too here. So that we can have some "threadsafe " state
	// in a blueprint; for example binary decomposition could store the coeffs ids.. .once.
	Blueprints []Blueprint
}

// GetNbConstraints returns the number of constraints
func (cs *NEWCS) GetNbConstraints() int {
	return cs.NbConstraints
}

func (cs *NEWCS) GetInstruction(i int) Instruction {
	return cs.Instructions[i]
}

func (cs *NEWCS) AddInstruction(blueprint Blueprint, inputs []uint32) (latestWire int) {
	// sanity check
	if debug.Debug {
		if blueprint.NbInputs() != len(inputs) {
			panic("blueprint.NbInputs() != len(inputs)")
		}
	}

	profile.RecordConstraint()

	instruction := Instruction{
		BlueprintID:           blueprint.UUID(),
		StartInternalVariable: uint32(cs.NbInternalVariables),
		StartCallData:         uint32(len(cs.CallData)),
	}
	cs.CallData = append(cs.CallData, inputs...)
	cs.Instructions = append(cs.Instructions, instruction)
	cs.NbConstraints += blueprint.NbConstraints()
	cs.NbInternalVariables += blueprint.NbWires()

	// TODO add profiling.

	// id of the last wire created by the blueprint.
	// TODO for simple "1 blueprint == 1 constraint" cases, this works.
	return cs.NbInternalVariables - 1
}

type TMPWireValueGetter interface {
	GetWireValue(i int) Coeff
	SetWireValue(i int, c Coeff)
}

type TMPCoeffGetter interface {
	// GetCoefficient returns coefficient with given id in the coeff table.
	// calls panic if i is out of bounds, because this is called in the hot path of the compiler.
	GetCoefficient(i int) Coeff
}

type BlueprintAddSCS struct {
}

var _ Blueprint = &BlueprintAddSCS{}

func (b *BlueprintAddSCS) UUID() uint32 {
	return 1 // TODO
}

func (b *BlueprintAddSCS) SolveFor(constraintOffset int, instruction Instruction, wirevalueGetter TMPWireValueGetter, coeffGetter TMPCoeffGetter) {
	// let's start in a first step with big.Int
	// missingWire := 42// some compute with b.internals and instructions
	// solver.GetCoefficient(..)
	// solver.GetWireValue(...)
	// solver.SetWireValue(...)
	// solution[missingWire] = (a*b + c*d + e*f) / q
	// xa := calldata[0]
	// xb := calldata[1]
	// qL := calldata[2]
	// qR := calldata[3]

	// vxa := solver.WireValue(xa)
	// vqL := solver.CoeffValue(qL)
	// vxa.Mul(vqL)

	// solver.SetWireValue(ID, vxa)
}
func (b *BlueprintAddSCS) NbInputs() int {
	return 4 // xa, xb, qL, qR
}
func (b *BlueprintAddSCS) NbConstraints() int {
	return 1
}
func (b *BlueprintAddSCS) NbWires() int {
	return 1
}

// in setup we want to iterate on constraint in logical order.
// we want wires and coeffs for one constraint.

func (b *BlueprintAddSCS) WriteSparseR1C(c *SparseR1C, constraintOffset int, instruction Instruction) {
	// c.Clear()
	// or we use coeff ids for now.
	// sparseR1C is short and sweet, can be a returned struct

	if debug.Debug {
		if constraintOffset != 0 {
			panic("invalid offset")
		}
	}

}

type Instruction struct {
	BlueprintID           uint32 // can add an extra byte for the commit stuff later.
	StartInternalVariable uint32 // internal variable start.
	StartCallData         uint32 // call data slice (end can be returned by Blueprint)
	_                     uint32 // future use
}

func (inst *Instruction) Blueprint() Blueprint {
	return BlueprintRegistry[inst.BlueprintID]
}

// Temporary
var BlueprintRegistry []Blueprint

type Blueprint interface {
	UUID() uint32
	SolveFor(constraintOffset int, instruction Instruction, wirevalueGetter TMPWireValueGetter, coeffGetter TMPCoeffGetter)
	NbInputs() int // to use to reslice NEWCS.calldata
	NbConstraints() int
	NbWires() int
	WriteSparseR1C(c *SparseR1C, constraintOffset int, instruction Instruction)
}

type BlueprintSCS interface {
	InstantiateConstraint(offset int) SparseR1C
}
