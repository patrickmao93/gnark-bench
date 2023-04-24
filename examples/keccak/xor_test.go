package keccak

import (
	"fmt"
	"math"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/test"
)

func TestXorLookup(t *testing.T) {
	circuit := &XorCircuit{}
	w := &XorCircuit{
		A: 3, // 11
		B: 1, // 01
		C: 2, // 10
	}
	err := test.IsSolved(circuit, w, ecc.BN254.ScalarField())
	check(err)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	check(err)
	fmt.Println("constraints", cs.GetNbConstraints())
}

type XorCircuit struct {
	A frontend.Variable
	B frontend.Variable
	C frontend.Variable
}

func (c *XorCircuit) Define(api frontend.API) error {
	table := logderivlookup.New(api)
	var k float64 = 8 // k + 1 quadruples the constraint count
	count := int(math.Pow(2, k))
	for _, v := range genXorTable(count) {
		table.Insert(v)
	}
	val := api.Add(api.Mul(c.A, count), c.B)
	xor := table.Lookup(val)
	api.AssertIsEqual(xor[0], c.C)
	return nil
}

func genXorTable(count int) []frontend.Variable {
	table := []frontend.Variable{}
	for i := 0; i < count; i++ {
		for j := 0; j < count; j++ {
			table = append(table, i^j)
		}
	}
	return table
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
