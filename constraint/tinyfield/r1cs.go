// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by gnark DO NOT EDIT

package cs

import (
	"errors"
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"io"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/internal/backend/ioutils"
	"github.com/consensys/gnark/logger"

	"github.com/consensys/gnark-crypto/ecc"

	fr "github.com/consensys/gnark/internal/tinyfield"
)

// R1CS describes a set of R1CS constraint
type R1CS struct {
	constraint.System
	CoeffTable
	arithEngine

	isR1CS bool
}

// NewR1CS returns a new R1CS and sets cs.Coefficient (fr.Element) from provided big.Int values
//
// capacity pre-allocates memory for capacity nbConstraints
func NewR1CS(capacity int) *R1CS {
	r := R1CS{
		System:     constraint.NewSystem(fr.Modulus(), capacity),
		CoeffTable: newCoeffTable(capacity / 10),
		isR1CS:     true,
	}
	return &r
}

// Solve solves the constraint system with provided witness.
// If it's a R1CS returns R1CSSolution
// If it's a SparseR1CS returns SparseR1CSSolution
func (cs *R1CS) Solve(witness witness.Witness, opts ...solver.Option) (any, error) {
	opt, err := solver.NewConfig(opts...)
	if err != nil {
		return nil, err
	}

	v := witness.Vector().(fr.Vector)

	solution, err := cs.solve(v, opt)
	if err != nil {
		return nil, err
	}

	if cs.isR1CS {
		var res R1CSSolution
		res.W = solution.values
		res.A = solution.a
		res.B = solution.b
		res.C = solution.c
		return &res, nil
	} else {
		// sparse R1CS
		var res SparseR1CSSolution
		// query l, r, o in Lagrange basis, not blinded
		res.L, res.R, res.O = cs.evaluateLROSmallDomain(solution.values)

		return &res, nil
	}

}

// Solve sets all the wires and returns the a, b, c vectors.
// the cs system should have been compiled before. The entries in a, b, c are in Montgomery form.
// a, b, c vectors: ab-c = hz
// witness = [publicWires | secretWires] (without the ONE_WIRE !)
// returns  [publicWires | secretWires | internalWires ]
func (cs *R1CS) solve(witness fr.Vector, opt solver.Config) (solution, error) {
	log := logger.Logger().With().Int("nbConstraints", cs.GetNbConstraints()).Logger()

	witnessOffset := 0
	if cs.isR1CS {
		witnessOffset++
	}

	nbWires := len(cs.Public) + len(cs.Secret) + cs.NbInternalVariables
	solution, err := newSolution(&cs.System, nbWires, opt.HintFunctions, cs.Coefficients, cs.isR1CS)
	if err != nil {
		return solution, err
	}
	start := time.Now()

	expectedWitnessSize := len(cs.Public) - witnessOffset + len(cs.Secret)

	if len(witness) != expectedWitnessSize {
		err = fmt.Errorf("invalid witness size, got %d, expected %d", len(witness), expectedWitnessSize)
		log.Err(err).Send()
		return solution, err
	}

	if witnessOffset == 1 {
		solution.solved[0] = true // ONE_WIRE
		solution.values[0].SetOne()
	}

	copy(solution.values[witnessOffset:], witness)
	for i := range witness {
		solution.solved[i+witnessOffset] = true
	}

	// keep track of the number of wire instantiations we do, for a sanity check to ensure
	// we instantiated all wires
	solution.nbSolved += uint64(len(witness) + witnessOffset)

	// now that we know all inputs are set, defer log printing once all solution.values are computed
	// (or sooner, if a constraint is not satisfied)
	defer solution.printLogs(opt.Logger, cs.Logs)

	if err := cs.parallelSolve(&solution); err != nil {
		if unsatisfiedErr, ok := err.(*UnsatisfiedConstraintError); ok {
			log.Err(errors.New("unsatisfied constraint")).Int("id", unsatisfiedErr.CID).Send()
		} else {
			log.Err(err).Send()
		}
		return solution, err
	}

	// sanity check; ensure all wires are marked as "instantiated"
	if !solution.isValid() {
		log.Err(errors.New("solver didn't instantiate all wires")).Send()
		panic("solver didn't instantiate all wires")
	}

	log.Debug().Dur("took", time.Since(start)).Msg("constraint system solver done")

	return solution, nil
}

func (cs *R1CS) solveInstruction(inst constraint.Instruction, solution *solution, tmpR1C *constraint.R1C, tmpSparseR1C *constraint.SparseR1C) error {
	blueprint := cs.Blueprints[inst.BlueprintID]

	if bc, ok := blueprint.(constraint.BlueprintHint); ok {
		var hm constraint.HintMapping
		bc.DecompressHint(&hm, cs.GetCallData(inst))
		return solution.solveWithHint(hm)
	}

	if bc, ok := blueprint.(constraint.BlueprintSolvable); ok {
		bc.Solve(solution, cs.GetCallData(inst))
	}

	if cs.isR1CS {
		if bc, ok := blueprint.(constraint.BlueprintR1C); ok {
			// TODO @gbotrel use pool object here for the R1C
			bc.DecompressR1C(tmpR1C, cs.GetCallData(inst))
			cID := inst.ConstraintOffset // here we have 1 constraint in the instruction only
			return cs.solveConstraint(cID, tmpR1C, solution)
		} else {
			// panic("not implemented")
		}
	} else {
		if bc, ok := blueprint.(constraint.BlueprintSparseR1C); ok {
			// sparse R1CS
			bc.DecompressSparseR1C(tmpSparseR1C, cs.GetCallData(inst))

			if err := cs.solveSparseR1C(tmpSparseR1C, solution); err != nil {
				return &UnsatisfiedConstraintError{CID: int(inst.ConstraintOffset), Err: err}
			}
			if err := cs.checkConstraint(tmpSparseR1C, solution); err != nil {
				return &UnsatisfiedConstraintError{CID: int(inst.ConstraintOffset), Err: err}
			}
			return nil
		} else {
			// panic("not implemented")
		}
	}

	return nil
}

func (cs *R1CS) parallelSolve(solution *solution) error {
	// minWorkPerCPU is the minimum target number of constraint a task should hold
	// in other words, if a level has less than minWorkPerCPU, it will not be parallelized and executed
	// sequentially without sync.
	const minWorkPerCPU = 50.0 // TODO @gbotrel revisit that with blocks.

	// cs.Levels has a list of levels, where all constraints in a level l(n) are independent
	// and may only have dependencies on previous levels
	// for each constraint
	// we are guaranteed that each R1C contains at most one unsolved wire
	// first we solve the unsolved wire (if any)
	// then we check that the constraint is valid
	// if a[i] * b[i] != c[i]; it means the constraint is not satisfied
	var wg sync.WaitGroup
	chTasks := make(chan []int, runtime.NumCPU())
	chError := make(chan error, runtime.NumCPU())

	// start a worker pool
	// each worker wait on chTasks
	// a task is a slice of constraint indexes to be solved
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			var r1c constraint.R1C
			var sparseR1C constraint.SparseR1C
			for t := range chTasks {
				for _, i := range t {
					// for each constraint in the task, solve it.
					// if err := cs.solveInstruction(cs.Instructions[i], solution, a, b, c); err != nil {
					if err := cs.solveInstruction(cs.Instructions[i], solution, &r1c, &sparseR1C); err != nil {
						// var debugInfo *string
						// if dID, ok := cs.MDebug[i]; ok {
						// 	debugInfo = new(string)
						// 	*debugInfo = solution.logValue(cs.DebugInfo[dID])
						// }
						chError <- err // TODO @gbotrel the only error we get here are from hints; make it typed.
						wg.Done()
						return
					}
				}
				wg.Done()
			}
		}()
	}

	// clean up pool go routines
	defer func() {
		close(chTasks)
		close(chError)
	}()

	// for each level, we push the tasks
	var r1c constraint.R1C
	var sparseR1C constraint.SparseR1C
	for _, level := range cs.Levels {

		// max CPU to use
		maxCPU := float64(len(level)) / minWorkPerCPU

		if maxCPU <= 1.0 {
			// we do it sequentially
			for _, i := range level {
				if err := cs.solveInstruction(cs.Instructions[i], solution, &r1c, &sparseR1C); err != nil {
					return err
				}
			}
			continue
		}

		// number of tasks for this level is set to number of CPU
		// but if we don't have enough work for all our CPU, it can be lower.
		nbTasks := runtime.NumCPU()
		maxTasks := int(math.Ceil(maxCPU))
		if nbTasks > maxTasks {
			nbTasks = maxTasks
		}
		nbIterationsPerCpus := len(level) / nbTasks

		// more CPUs than tasks: a CPU will work on exactly one iteration
		// note: this depends on minWorkPerCPU constant
		if nbIterationsPerCpus < 1 {
			nbIterationsPerCpus = 1
			nbTasks = len(level)
		}

		extraTasks := len(level) - (nbTasks * nbIterationsPerCpus)
		extraTasksOffset := 0

		for i := 0; i < nbTasks; i++ {
			wg.Add(1)
			_start := i*nbIterationsPerCpus + extraTasksOffset
			_end := _start + nbIterationsPerCpus
			if extraTasks > 0 {
				_end++
				extraTasks--
				extraTasksOffset++
			}
			// since we're never pushing more than num CPU tasks
			// we will never be blocked here
			chTasks <- level[_start:_end]
		}

		// wait for the level to be done
		wg.Wait()

		if len(chError) > 0 {
			return <-chError
		}
	}

	return nil
}

// IsSolved
// Deprecated: use _, err := Solve(...) instead
func (cs *R1CS) IsSolved(witness witness.Witness, opts ...solver.Option) error {
	_, err := cs.Solve(witness, opts...)
	return err
}

// divByCoeff sets res = res / t.Coeff
func (cs *R1CS) divByCoeff(res *fr.Element, t constraint.Term) {
	cID := t.CoeffID()
	switch cID {
	case constraint.CoeffIdOne:
		return
	case constraint.CoeffIdMinusOne:
		res.Neg(res)
	case constraint.CoeffIdZero:
		panic("division by 0")
	default:
		// this is slow, but shouldn't happen as divByCoeff is called to
		// remove the coeff of an unsolved wire
		// but unsolved wires are (in gnark frontend) systematically set with a coeff == 1 or -1
		res.Div(res, &cs.Coefficients[cID])
	}
}

func (cs *R1CS) wrapErrWithDebugInfo(solution *solution, cID uint32, err error) *UnsatisfiedConstraintError {
	var debugInfo *string
	if dID, ok := cs.MDebug[int(cID)]; ok {
		debugInfo = new(string)
		*debugInfo = solution.logValue(cs.DebugInfo[dID])
	}
	if err == nil {
		err = errors.New("unsatisfied constraint error")
	}
	return &UnsatisfiedConstraintError{CID: int(cID), Err: err, DebugInfo: debugInfo}
}

// solveConstraint compute unsolved wires in the constraint, if any and set the solution accordingly
//
// returns an error if the solver called a hint function that errored
// returns false, nil if there was no wire to solve
// returns true, nil if exactly one wire was solved. In that case, it is redundant to check that
// the constraint is satisfied later.
func (cs *R1CS) solveConstraint(cID uint32, r *constraint.R1C, solution *solution) error {
	a, b, c := &solution.a[cID], &solution.b[cID], &solution.c[cID]

	// the index of the non-zero entry shows if L, R or O has an uninstantiated wire
	// the content is the ID of the wire non instantiated
	var loc uint8

	var termToCompute constraint.Term

	processLExp := func(l constraint.LinearExpression, val *fr.Element, locValue uint8) {
		for _, t := range l {
			vID := t.WireID()

			// wire is already computed, we just accumulate in val
			if solution.solved[vID] {
				solution.accumulateInto(t, val)
				continue
			}

			if loc != 0 {
				panic("found more than one wire to instantiate")
			}
			termToCompute = t
			loc = locValue
		}
	}

	processLExp(r.L, a, 1)
	processLExp(r.R, b, 2)
	processLExp(r.O, c, 3)

	if loc == 0 {
		// there is nothing to solve, may happen if we have an assertion
		// (ie a constraints that doesn't yield any output)
		// or if we solved the unsolved wires with hint functions
		var check fr.Element
		if !check.Mul(a, b).Equal(c) {
			return cs.wrapErrWithDebugInfo(solution, cID, fmt.Errorf("%s ⋅ %s != %s", a.String(), b.String(), c.String()))
		}
		return nil
	}

	// we compute the wire value and instantiate it
	wID := termToCompute.WireID()

	// solver result
	var wire fr.Element

	switch loc {
	case 1:
		if !b.IsZero() {
			wire.Div(c, b).
				Sub(&wire, a)
			a.Add(a, &wire)
		} else {
			// we didn't actually ensure that a * b == c
			var check fr.Element
			if !check.Mul(a, b).Equal(c) {
				return cs.wrapErrWithDebugInfo(solution, cID, fmt.Errorf("%s ⋅ %s != %s", a.String(), b.String(), c.String()))
			}
		}
	case 2:
		if !a.IsZero() {
			wire.Div(c, a).
				Sub(&wire, b)
			b.Add(b, &wire)
		} else {
			var check fr.Element
			if !check.Mul(a, b).Equal(c) {
				return cs.wrapErrWithDebugInfo(solution, cID, fmt.Errorf("%s ⋅ %s != %s", a.String(), b.String(), c.String()))
			}
		}
	case 3:
		wire.Mul(a, b).
			Sub(&wire, c)

		c.Add(c, &wire)
	}

	// wire is the term (coeff * value)
	// but in the solution we want to store the value only
	// note that in gnark frontend, coeff here is always 1 or -1
	cs.divByCoeff(&wire, termToCompute)
	solution.set(wID, wire)

	return nil
}

// GetConstraints return the list of R1C and a coefficient resolver
func (cs *R1CS) GetConstraints() ([]constraint.R1C, constraint.Resolver) {
	toReturn := make([]constraint.R1C, 0, cs.GetNbConstraints())

	for _, inst := range cs.Instructions {
		blueprint := cs.Blueprints[inst.BlueprintID]
		if bc, ok := blueprint.(constraint.BlueprintR1C); ok {
			var r1c constraint.R1C
			bc.DecompressR1C(&r1c, cs.GetCallData(inst))
			toReturn = append(toReturn, r1c)
		} else {
			panic("not implemented")
		}
	}
	return toReturn, cs
}

// GetNbCoefficients return the number of unique coefficients needed in the R1CS
func (cs *R1CS) GetNbCoefficients() int {
	return len(cs.Coefficients)
}

// CurveID returns curve ID as defined in gnark-crypto
func (cs *R1CS) CurveID() ecc.ID {
	return ecc.UNKNOWN
}

// WriteTo encodes R1CS into provided io.Writer using cbor
func (cs *R1CS) WriteTo(w io.Writer) (int64, error) {
	_w := ioutils.WriterCounter{W: w} // wraps writer to count the bytes written
	enc, err := cbor.CoreDetEncOptions().EncMode()
	if err != nil {
		return 0, err
	}
	encoder := enc.NewEncoder(&_w)

	// encode our object
	err = encoder.Encode(cs)
	return _w.N, err
}

// ReadFrom attempts to decode R1CS from io.Reader using cbor
func (cs *R1CS) ReadFrom(r io.Reader) (int64, error) {
	dm, err := cbor.DecOptions{
		MaxArrayElements: 134217728,
		MaxMapPairs:      134217728,
	}.DecMode()

	if err != nil {
		return 0, err
	}
	decoder := dm.NewDecoder(r)

	// initialize coeff table
	cs.CoeffTable = newCoeffTable(0)

	if err := decoder.Decode(&cs); err != nil {
		return int64(decoder.NumBytesRead()), err
	}

	if err := cs.CheckSerializationHeader(); err != nil {
		return int64(decoder.NumBytesRead()), err
	}

	return int64(decoder.NumBytesRead()), nil
}

// evaluateLROSmallDomain extracts the solution l, r, o, and returns it in lagrange form.
// solution = [ public | secret | internal ]
func (cs *R1CS) evaluateLROSmallDomain(solution []fr.Element) ([]fr.Element, []fr.Element, []fr.Element) {

	//s := int(pk.Domain[0].Cardinality)
	s := cs.GetNbConstraints() + len(cs.Public) // len(spr.Public) is for the placeholder constraints
	s = int(ecc.NextPowerOfTwo(uint64(s)))

	var l, r, o []fr.Element
	l = make([]fr.Element, s)
	r = make([]fr.Element, s)
	o = make([]fr.Element, s)
	s0 := solution[0]

	for i := 0; i < len(cs.Public); i++ { // placeholders
		l[i] = solution[i]
		r[i] = s0
		o[i] = s0
	}
	offset := len(cs.Public)
	nbConstraints := cs.GetNbConstraints()

	var sparseR1C constraint.SparseR1C
	j := 0
	for _, inst := range cs.Instructions {
		blueprint := cs.Blueprints[inst.BlueprintID]
		if bc, ok := blueprint.(constraint.BlueprintSparseR1C); ok {
			bc.DecompressSparseR1C(&sparseR1C, cs.GetCallData(inst))

			l[offset+j] = solution[sparseR1C.XA]
			r[offset+j] = solution[sparseR1C.XB]
			o[offset+j] = solution[sparseR1C.XC]
			j++
		} else {
			// TODO need to handle block of constraints.
			// panic("not implemented")
		}
	}

	offset += nbConstraints

	for i := 0; i < s-offset; i++ { // offset to reach 2**n constraints (where the id of l,r,o is 0, so we assign solution[0])
		l[offset+i] = s0
		r[offset+i] = s0
		o[offset+i] = s0
	}

	return l, r, o

}
