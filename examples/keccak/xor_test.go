package keccak

import (
	"fmt"
	"math"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/profile"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/test"
)

func TestXorLookup(t *testing.T) {
	// w := &XorCircuit{
	// 	A: 6,  // 0110
	// 	B: 9,  // 1001
	// 	C: 15, // 1111

	// 	// D: 7,  // 0111
	// 	// E: 9,  // 1001
	// 	// F: 14, // 1110

	// 	XorBits: 16,
	// }
	circuit := &XorCircuit{
		XorBits: 16,
	}
	// err := test.IsSolved(circuit, w, ecc.BN254.ScalarField())
	// check(err)
	p := profile.Start()
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	check(err)
	p.Stop()
	fmt.Println(p.Top())
	fmt.Println("constraints", cs.GetNbConstraints())
}

type XorCircuit struct {
	A frontend.Variable
	B frontend.Variable
	C frontend.Variable

	// D frontend.Variable
	// E frontend.Variable
	// F frontend.Variable

	XorBits int
}

func (c *XorCircuit) Define(api frontend.API) error {
	table := logderivlookup.New(api)
	for _, v := range genXorTable(c.XorBits) {
		// fmt.Printf("key %08b, val %04b\n", i, v)
		table.Insert(v)
	}
	shiftedA := api.Mul(c.A, int(math.Pow(2, float64(c.XorBits))))
	i := api.Add(shiftedA, c.B)
	xor := table.Lookup(i)
	api.AssertIsEqual(xor[0], c.C)

	// shiftedD := api.Mul(c.D, int(math.Pow(2, float64(c.XorBits))))
	// input2 := api.Add(shiftedD, c.E)
	// for i := 0; i < 1; i++ {
	// 	xor := table.Lookup(input2)
	// 	api.AssertIsEqual(xor[0], c.F)
	// }

	return nil
}

func TestXorLookup2(t *testing.T) {
	w := &XorCircuit2{
		A: 6,  // 0110
		B: 9,  // 1001
		C: 15, // 1111

		XorBits: 8,
	}
	circuit := &XorCircuit2{
		XorBits: 8,
	}
	err := test.IsSolved(circuit, w, ecc.BN254.ScalarField())
	check(err)
	p := profile.Start()
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	check(err)
	p.Stop()
	fmt.Println(p.Top())
	fmt.Println("constraints", cs.GetNbConstraints())
}

type XorCircuit2 struct {
	A frontend.Variable
	B frontend.Variable
	C frontend.Variable

	XorBits int
}

func (c *XorCircuit2) Define(api frontend.API) error {
	table := logderivlookup.New(api)
	for _, v := range genXorTable(c.XorBits) {
		// fmt.Printf("key %08b, val %04b\n", i, v)
		table.Insert(v)
	}
	shiftedA := api.Mul(c.A, int(math.Pow(2, float64(c.XorBits))))
	input := api.Add(shiftedA, c.B)
	for i := 0; i < 2; i++ {
		xor := table.Lookup(input)
		api.AssertIsEqual(xor[0], c.C)
	}
	return nil
}

func genXorTable(inBits int) []frontend.Variable {
	table := []frontend.Variable{}
	k := int(math.Pow(2, float64(inBits)))
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			table = append(table, i^j)
		}
	}
	return table
}

func TestXorOld(t *testing.T) {
	w := &XorOldCircuit{
		A: 6,  // 0110
		B: 9,  // 1001
		C: 15, // 1111
	}
	circuit := &XorOldCircuit{}
	err := test.IsSolved(circuit, w, ecc.BN254.ScalarField())
	check(err)
	p := profile.Start()
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	check(err)
	p.Stop()
	fmt.Println(p.Top())
	fmt.Println("constraints", cs.GetNbConstraints())
}

type XorOldCircuit struct {
	A frontend.Variable
	B frontend.Variable
	C frontend.Variable
}

func (c *XorOldCircuit) Define(api frontend.API) error {
	for i := 0; i < 1023; i++ {
		a := api.ToBinary(c.A, 64)
		b := api.ToBinary(c.B, 64)
		xor := []frontend.Variable{}
		for i := 0; i < 64; i++ {
			xor = append(xor, api.Xor(a[i], b[i]))
		}
		api.AssertIsEqual(api.FromBinary(xor...), c.C)
	}
	return nil
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
