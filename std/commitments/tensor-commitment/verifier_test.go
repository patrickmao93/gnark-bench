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

package tensorcommitment

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/sis"
	tensorcommitment "github.com/consensys/gnark-crypto/ecc/bn254/fr/tensor-commitment"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	gsis "github.com/consensys/gnark/std/hash/sis"
)

// evalAtPower subroutine
type EvalAtPower struct {

	// polynomial
	P [16]frontend.Variable

	// variable at which P is evaluated
	X big.Int

	// size of the polynomial
	size uint64

	// exponent n at which we compute P(X^{n})
	N frontend.Variable

	// expected result P(X^{n})
	R frontend.Variable
}

func (circuit *EvalAtPower) Define(api frontend.API) error {

	r := evalAtPower(api, circuit.P[:], circuit.X, circuit.N, circuit.size)
	api.AssertIsEqual(r, circuit.R)

	return nil
}

func printPoly(p []fr.Element) {

	for i := 0; i < len(p)-1; i++ {
		fmt.Printf("%s*x**%d+", p[i].String(), i)
	}
	fmt.Printf("%s*x**%d\n", p[len(p)-1].String(), len(p)-1)

}

func TestEvalAtPower(t *testing.T) {

	// generate random polynomial
	var p [16]fr.Element
	for i := 0; i < 16; i++ {
		p[i].SetRandom()
	}

	// pick arbitrary point at which p is evaluated
	var x fr.Element
	x.SetRandom()

	// arbitrary exponent
	var e big.Int
	e.SetUint64(189)

	// exponentiate x mod BN254 scalar field
	var xexp fr.Element
	xexp.Exp(x, &e)

	// calcuate p(x^{e})
	var res fr.Element
	res.SetUint64(0)
	for i := 0; i < len(p); i++ {
		res.Mul(&res, &xexp)
		res.Add(&res, &p[len(p)-1-i])
	}

	// create the witness
	var witness EvalAtPower
	for i := 0; i < 16; i++ {
		witness.P[i] = p[i].String()
	}
	x.ToBigIntRegular(&witness.X)
	witness.size = 16
	witness.N = e.String()
	witness.R = res.String()

	// create the circuit
	var circuit EvalAtPower
	circuit.size = 16
	x.ToBigIntRegular(&circuit.X)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// check if the solving is correct
	twitness, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// subroutine for selecting an entry whose index is a frontend.Variable
type SelectEntry struct {
	Tab [][]frontend.Variable
	E   frontend.Variable
	R   []frontend.Variable
}

func (circuit *SelectEntry) Define(api frontend.API) error {

	r := selectEntry(api, circuit.E, circuit.Tab)
	for i := 0; i < len(circuit.R); i++ {
		api.AssertIsEqual(r[i], circuit.R[i])
	}

	return nil
}

func TestSelectEntry(t *testing.T) {

	rows := 10
	columns := 5

	tab := make([][]int, rows)
	for i := 0; i < rows; i++ {
		tab[i] = make([]int, columns)
		for j := 0; j < columns; j++ {
			tab[i][j] = 10*i + j
		}
	}

	var witness SelectEntry
	witness.Tab = make([][]frontend.Variable, rows)
	for i := 0; i < rows; i++ {
		witness.Tab[i] = make([]frontend.Variable, columns)
		for j := 0; j < columns; j++ {
			witness.Tab[i][j] = tab[i][j]
		}
	}
	witness.R = make([]frontend.Variable, columns)

	// compile the circuit
	var circuit SelectEntry
	circuit.Tab = make([][]frontend.Variable, rows)
	for i := 0; i < rows; i++ {
		circuit.Tab[i] = make([]frontend.Variable, columns)
	}
	circuit.R = make([]frontend.Variable, columns)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < rows; i++ {

		// create the witness
		witness.E = i
		for j := 0; j < columns; j++ {
			witness.R[j] = tab[i][j]
		}
		twitness, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			t.Fatal(err)
		}

		err = ccs.IsSolved(twitness)
		if err != nil {
			t.Fatal(err)
		}

	}

}

// Verifier circuit
type Verifier struct {

	// Polynomial committed
	Digest [][]frontend.Variable

	// variables used in the proof
	EntryList         []frontend.Variable
	Columns           [][]frontend.Variable
	LinearCombination []frontend.Variable

	// random coefficients sent by the verifier
	L []frontend.Variable

	// Domain used in the tensor commitment
	SizeDomainTensorCommitment uint64

	// Generator of the domain used in the tensor commitment
	GenDomainTensorCommitment big.Int

	// hash function
	Sis sis.RSis
}

func (circuit *Verifier) Define(api frontend.API) error {

	// proof
	var proof Proof
	proof.EntryList = circuit.EntryList
	proof.Columns = circuit.Columns
	proof.LinearCombination = circuit.LinearCombination
	proof.SizeDomainTensorCommitment = circuit.SizeDomainTensorCommitment
	proof.GenDomainTensorCommitment = circuit.GenDomainTensorCommitment

	// snark version of sis
	sisSnark := gsis.NewRSisSnark(circuit.Sis)

	return Verify(api, proof, circuit.Digest, circuit.L, sisSnark)

}

func TestTensorCommitment(t *testing.T) {

	var rho, size, sqrtSize int
	rho = 4
	size = 64
	sqrtSize = 8

	logTwoDegree := 1
	logTwoBound := 4
	keySize := 256
	h, err := sis.NewRSis(5, logTwoDegree, logTwoBound, keySize)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := tensorcommitment.NewTensorCommitment(rho, size, h)
	if err != nil {
		t.Fatal(err)
	}

	// random polynomial
	p := make([]fr.Element, size)
	for i := 0; i < size; i++ {
		p[i].SetRandom()
	}

	// coefficients for the linear combination
	l := make([]fr.Element, sqrtSize)
	for i := 0; i < sqrtSize; i++ {
		l[i].SetRandom()
	}

	// we select all the entries for the test
	entryList := make([]int, rho*sqrtSize)
	for i := 0; i < rho*sqrtSize; i++ {
		entryList[i] = i
	}

	// compute the digest...
	digest, err := tc.Commit(p)
	if err != nil {
		t.Fatal(err)
	}

	// build the proof...
	proof, err := tc.BuildProof(p, l, entryList)
	if err != nil {
		t.Fatal(err)
	}

	// verfiy that the proof is correct
	err = tensorcommitment.Verify(proof, digest, l, h)
	if err != nil {
		t.Fatal(err)
	}

	// now we have everything to populate the witness
	var witness Verifier

	_h := *(h.(*sis.RSis))
	witness.Digest = make([][]frontend.Variable, len(digest))
	var tmp fr.Element
	nbBytesFr := 32
	for i := 0; i < len(digest); i++ {
		witness.Digest[i] = make([]frontend.Variable, _h.Degree)
		for j := 0; j < _h.Degree; j++ {
			tmp.SetBytes(digest[i][j*nbBytesFr : (j+1)*nbBytesFr])
			witness.Digest[i][j] = tmp.String()
		}
	}

	witness.EntryList = make([]frontend.Variable, len(entryList))
	for i := 0; i < len(entryList); i++ {
		witness.EntryList[i] = entryList[i]
	}

	witness.Columns = make([][]frontend.Variable, len(proof.Columns))
	for i := 0; i < len(proof.Columns); i++ {
		witness.Columns[i] = make([]frontend.Variable, len(proof.Columns[i]))
		for j := 0; j < len(witness.Columns[i]); j++ {
			witness.Columns[i][j] = proof.Columns[i][j].String()
		}
	}

	witness.LinearCombination = make([]frontend.Variable, len(proof.LinearCombination))
	for i := 0; i < len(witness.LinearCombination); i++ {
		witness.LinearCombination[i] = proof.LinearCombination[i].String()
	}

	witness.L = make([]frontend.Variable, sqrtSize)
	for i := 0; i < sqrtSize; i++ {
		witness.L[i] = l[i].String()
	}
	witness.SizeDomainTensorCommitment = tc.Domain.Cardinality
	tc.Domain.Generator.ToBigIntRegular(&witness.GenDomainTensorCommitment)
	witness.Sis = _h

	// create the circuit
	var circuit Verifier
	circuit.Digest = make([][]frontend.Variable, len(digest))
	for i := 0; i < len(digest); i++ {
		circuit.Digest[i] = make([]frontend.Variable, _h.Degree)
	}
	circuit.EntryList = make([]frontend.Variable, len(entryList))
	circuit.Columns = make([][]frontend.Variable, len(proof.Columns))
	for i := 0; i < len(proof.Columns); i++ {
		circuit.Columns[i] = make([]frontend.Variable, len(proof.Columns[i]))
	}
	circuit.LinearCombination = make([]frontend.Variable, len(proof.LinearCombination))
	circuit.L = make([]frontend.Variable, sqrtSize)
	circuit.SizeDomainTensorCommitment = tc.Domain.Cardinality
	tc.Domain.Generator.ToBigIntRegular(&circuit.GenDomainTensorCommitment)
	circuit.Sis = _h

	// compile...
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// check the solving
	twitness, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("nb constraints: %d\n", ccs.GetNbConstraints())

}