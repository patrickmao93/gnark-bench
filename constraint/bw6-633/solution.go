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
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/debug"
	"github.com/rs/zerolog"
	"io"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/consensys/gnark-crypto/ecc/bw6-633/fr"
)

// solution represents elements needed to compute
// a solution to a R1CS or SparseR1CS
type solution struct {
	arithEngine
	values, coefficients []fr.Element
	solved               []bool
	nbSolved             uint64
	mHintsFunctions      map[solver.HintID]solver.Hint // maps hintID to hint function
	st                   *debug.SymbolTable
	cs                   *constraint.System
}

func (s *solution) GetValue(cID, vID uint32) constraint.Coeff {
	var r constraint.Coeff
	e := s.computeTerm(constraint.Term{CID: cID, VID: vID})
	copy(r[:], e[:])
	return r
}
func (s *solution) GetCoeff(cID uint32) constraint.Coeff {
	var r constraint.Coeff
	copy(r[:], s.coefficients[cID][:])
	return r
}
func (s *solution) SetValue(vID uint32, f constraint.Coeff) {
	s.set(int(vID), fr.Element(f[:]))
}

// type felt struct {
// 	fr.Element
// }

// func anyToElement(other any) fr.Element {
// 	if f, ok := other.(*felt); ok {
// 		return f.Element
// 	}
// 	var o fr.Element
// 	if _, err := o.SetInterface(other); err != nil {
// 		panic(err)
// 	}
// 	return o
// }

// func (f *felt) Mul(other any) {
// 	o := anyToElement(other)
// 	f.Element.Mul(&f.Element, &o)
// }
// func (f *felt) Div(other any) {
// 	o := anyToElement(other)
// 	f.Element.Div(&f.Element, &o)
// }
// func (f *felt) Add(other any) {
// 	o := anyToElement(other)
// 	f.Element.Add(&f.Element, &o)
// }
// func (f *felt) Sub(other any) {
// 	o := anyToElement(other)
// 	f.Element.Sub(&f.Element, &o)
// }

// type Solver interface {
// 	GetValue(cID, vID uint32) Felt
// 	GetCoeff(cID uint32) Felt
// 	SetValue(vID uint32, f Felt)
// 	TmpFelt() Felt
// }

// type Felt interface {
// 	Mul(other any)
// 	Div(other any)
// 	Add(other any)
// 	Sub(other any)
// }

func (s *solution) GetWireValue(i int) constraint.Coeff {
	var c constraint.Coeff
	copy(c[:], s.values[i][:])
	return c
}

func (s *solution) SetWireValue(i int, c constraint.Coeff) {
	var e fr.Element
	copy(e[:], c[:])
	s.set(i, e)
}

func newSolution(cs *constraint.System, nbWires int, hintFunctions map[solver.HintID]solver.Hint, coefficients []fr.Element) (solution, error) {

	s := solution{
		cs:              cs,
		st:              &cs.SymbolTable,
		values:          make([]fr.Element, nbWires),
		coefficients:    coefficients,
		solved:          make([]bool, nbWires),
		mHintsFunctions: hintFunctions,
	}

	// hintsDependencies is from compile time; it contains the list of hints the solver **needs**
	var missing []string
	for hintUUID, hintID := range cs.MHintsDependencies {
		if _, ok := s.mHintsFunctions[hintUUID]; !ok {
			missing = append(missing, hintID)
		}
	}

	if len(missing) > 0 {
		return s, fmt.Errorf("solver missing hint(s): %v", missing)
	}

	return s, nil
}

func (s *solution) set(id int, value fr.Element) {
	if s.solved[id] {
		panic("solving the same wire twice should never happen.")
	}
	s.values[id] = value
	s.solved[id] = true
	atomic.AddUint64(&s.nbSolved, 1)
	// s.nbSolved++
}

func (s *solution) isValid() bool {
	return int(s.nbSolved) == len(s.values)
}

// computeTerm computes coeff*variable
func (s *solution) computeTerm(t constraint.Term) fr.Element {
	cID, vID := t.CoeffID(), t.WireID()
	if cID != 0 && !s.solved[vID] {
		panic("computing a term with an unsolved wire")
	}
	switch cID {
	case constraint.CoeffIdZero:
		return fr.Element{}
	case constraint.CoeffIdOne:
		return s.values[vID]
	case constraint.CoeffIdTwo:
		var res fr.Element
		res.Double(&s.values[vID])
		return res
	case constraint.CoeffIdMinusOne:
		var res fr.Element
		res.Neg(&s.values[vID])
		return res
	default:
		var res fr.Element
		res.Mul(&s.coefficients[cID], &s.values[vID])
		return res
	}
}

// r += (t.coeff*t.value)
func (s *solution) accumulateInto(t constraint.Term, r *fr.Element) {
	cID := t.CoeffID()
	vID := t.WireID()

	// if t.IsConstant() {
	// 	// needed for logs, we may want to not put this in the hot path if we need to
	// 	// optimize constraint system solver further.
	// 	r.Add(r, &s.coefficients[cID])
	// 	return
	// }

	switch cID {
	case constraint.CoeffIdZero:
		return
	case constraint.CoeffIdOne:
		r.Add(r, &s.values[vID])
	case constraint.CoeffIdTwo:
		var res fr.Element
		res.Double(&s.values[vID])
		r.Add(r, &res)
	case constraint.CoeffIdMinusOne:
		r.Sub(r, &s.values[vID])
	default:
		var res fr.Element
		res.Mul(&s.coefficients[cID], &s.values[vID])
		r.Add(r, &res)
	}
}

// solveHint compute solution.values[vID] using provided solver hint
func (s *solution) solveWithHint(h constraint.HintMapping) error {
	// skip if the wire is already solved by a call to the same hint
	// function on the same inputs
	// TODO sanity check this shouldn't happen.
	// if s.solved[vID] {
	//     return nil
	// }

	// ensure hint function was provided
	f, ok := s.mHintsFunctions[h.HintID]
	if !ok {
		return errors.New("missing hint function")
	}

	// tmp IO big int memory
	nbInputs := len(h.Inputs)
	nbOutputs := len(h.Outputs)
	inputs := make([]*big.Int, nbInputs)
	outputs := make([]*big.Int, nbOutputs)
	for i := 0; i < nbOutputs; i++ {
		outputs[i] = big.NewInt(0)
	}

	q := fr.Modulus()

	// // for each input, we set its big int value, IF all the wires are solved
	// // the only case where all wires may not be solved, is if one of the input of this hint
	// // is the output of another hint.
	// // it is safe to recursively solve this with the parallel solver, since all hints-output wires
	// // that we can solve this way are marked to be solved with the current constraint we are processing.
	// recursiveSolve := func(t constraint.Term) error {
	// 	if t.IsConstant() {
	// 		return nil
	// 	}
	// 	wID := t.WireID()
	// 	if s.solved[wID] {
	// 		return nil
	// 	}
	// 	// unsolved dependency
	// 	if h, ok := s.cs.MHints[wID]; ok {
	// 		// solve recursively.
	// 		return s.solveWithHint(wID, h)
	// 	}

	// 	// it's not a hint, we panic.
	// 	panic("solver can't compute hint; one or more input wires are unsolved")
	// }

	for i := 0; i < nbInputs; i++ {
		inputs[i] = big.NewInt(0)

		var v fr.Element
		for _, term := range h.Inputs[i] {
			if term.IsConstant() {
				v.Add(&v, &s.coefficients[term.CoeffID()])
				continue
			}
			s.accumulateInto(term, &v)
		}
		v.BigInt(inputs[i])
	}

	err := f(q, inputs, outputs)

	var v fr.Element
	for i := range outputs {
		v.SetBigInt(outputs[i])
		s.set(h.Outputs[i], v)
	}

	return err
}

func (s *solution) printLogs(log zerolog.Logger, logs []constraint.LogEntry) {
	if log.GetLevel() == zerolog.Disabled {
		return
	}

	for i := 0; i < len(logs); i++ {
		logLine := s.logValue(logs[i])
		log.Debug().Str(zerolog.CallerFieldName, logs[i].Caller).Msg(logLine)
	}
}

const unsolvedVariable = "<unsolved>"

func (s *solution) logValue(log constraint.LogEntry) string {
	var toResolve []interface{}
	var (
		eval         fr.Element
		missingValue bool
	)
	for j := 0; j < len(log.ToResolve); j++ {
		// before eval le

		missingValue = false
		eval.SetZero()

		for _, t := range log.ToResolve[j] {
			// for each term in the linear expression

			cID, vID := t.CoeffID(), t.WireID()
			if t.IsConstant() {
				// just add the constant
				eval.Add(&eval, &s.coefficients[cID])
				continue
			}

			if !s.solved[vID] {
				missingValue = true
				break // stop the loop we can't evaluate.
			}

			tv := s.computeTerm(t)
			eval.Add(&eval, &tv)
		}

		// after
		if missingValue {
			toResolve = append(toResolve, unsolvedVariable)
		} else {
			// we have to append our accumulator
			toResolve = append(toResolve, eval.String())
		}

	}
	if len(log.Stack) > 0 {
		var sbb strings.Builder
		for _, lID := range log.Stack {
			location := s.st.Locations[lID]
			function := s.st.Functions[location.FunctionID]

			sbb.WriteString(function.Name)
			sbb.WriteByte('\n')
			sbb.WriteByte('\t')
			sbb.WriteString(function.Filename)
			sbb.WriteByte(':')
			sbb.WriteString(strconv.Itoa(int(location.Line)))
			sbb.WriteByte('\n')
		}
		toResolve = append(toResolve, sbb.String())
	}
	return fmt.Sprintf(log.Format, toResolve...)
}

// UnsatisfiedConstraintError wraps an error with useful metadata on the unsatisfied constraint
type UnsatisfiedConstraintError struct {
	Err       error
	CID       int     // constraint ID
	DebugInfo *string // optional debug info
}

func (r *UnsatisfiedConstraintError) Error() string {
	if r.DebugInfo != nil {
		return fmt.Sprintf("constraint #%d is not satisfied: %s", r.CID, *r.DebugInfo)
	}
	return fmt.Sprintf("constraint #%d is not satisfied: %s", r.CID, r.Err.Error())
}

// R1CSSolution represent a valid assignment to all the variables in the constraint system.
// The vector W such that Aw o Bw - Cw = 0
type R1CSSolution struct {
	W       fr.Vector
	A, B, C fr.Vector
}

func (t *R1CSSolution) WriteTo(w io.Writer) (int64, error) {
	n, err := t.W.WriteTo(w)
	if err != nil {
		return n, err
	}
	a, err := t.A.WriteTo(w)
	n += a
	if err != nil {
		return n, err
	}
	a, err = t.B.WriteTo(w)
	n += a
	if err != nil {
		return n, err
	}
	a, err = t.C.WriteTo(w)
	n += a
	return n, err
}

func (t *R1CSSolution) ReadFrom(r io.Reader) (int64, error) {
	n, err := t.W.ReadFrom(r)
	if err != nil {
		return n, err
	}
	a, err := t.A.ReadFrom(r)
	a += n
	if err != nil {
		return n, err
	}
	a, err = t.B.ReadFrom(r)
	a += n
	if err != nil {
		return n, err
	}
	a, err = t.C.ReadFrom(r)
	n += a
	return n, err
}

// SparseR1CSSolution represent a valid assignment to all the variables in the constraint system.
type SparseR1CSSolution struct {
	L, R, O fr.Vector
}

func (t *SparseR1CSSolution) WriteTo(w io.Writer) (int64, error) {
	n, err := t.L.WriteTo(w)
	if err != nil {
		return n, err
	}
	a, err := t.R.WriteTo(w)
	n += a
	if err != nil {
		return n, err
	}
	a, err = t.O.WriteTo(w)
	n += a
	return n, err

}

func (t *SparseR1CSSolution) ReadFrom(r io.Reader) (int64, error) {
	n, err := t.L.ReadFrom(r)
	if err != nil {
		return n, err
	}
	a, err := t.R.ReadFrom(r)
	a += n
	if err != nil {
		return n, err
	}
	a, err = t.O.ReadFrom(r)
	a += n
	return n, err
}
