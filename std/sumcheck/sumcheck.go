package sumcheck

import (
	"fmt"
	"github.com/consensys/gnark/frontend"
)

type Polynomial []frontend.Variable //TODO: Is there already such a data structure?

// LazyClaims is the Claims data structure on the verifier side. It is "lazy" in that it has to compute fewer things.
type LazyClaims interface {
	ClaimsNum() int                                    // ClaimsNum = m
	VarsNum() int                                      // VarsNum = n
	CombinedSum(a frontend.Variable) frontend.Variable // CombinedSum returns c = ∑_{1≤j≤m} aʲ⁻¹cⱼ
	Degree(i int) int                                  //Degree of the total claim in the i'th variable
	VerifyFinalEval(api frontend.API, r []frontend.Variable, combinationCoeff, purportedValue frontend.Variable, proof interface{}) error
}

// Proof of a multi-sumcheck statement.
type Proof struct {
	PartialSumPolys []Polynomial
	FinalEvalProof  interface{} //in case it is difficult for the verifier to compute g(r₁, ..., rₙ) on its own, the prover can provide the value and a proof
}

func Verify(api frontend.API, claims LazyClaims, proof Proof, transcript ArithmeticTranscript) error {
	var combinationCoeff frontend.Variable

	if claims.ClaimsNum() >= 2 {
		combinationCoeff = transcript.Next()
	}

	r := make([]frontend.Variable, claims.VarsNum())

	// Just so that there is enough room for gJ to be reused
	maxDegree := claims.Degree(0)
	for j := 1; j < claims.VarsNum(); j++ {
		if d := claims.Degree(j); d > maxDegree {
			maxDegree = d
		}
	}

	gJ := make(Polynomial, maxDegree+1)         //At the end of iteration j, gJ = ∑_{i < 2ⁿ⁻ʲ⁻¹} g(X₁, ..., Xⱼ₊₁, i...)		NOTE: n is shorthand for claims.VarsNum()
	gJR := claims.CombinedSum(combinationCoeff) // At the beginning of iteration j, gJR = ∑_{i < 2ⁿ⁻ʲ} g(r₁, ..., rⱼ, i...)

	for j := 0; j < claims.VarsNum(); j++ {
		if len(proof.PartialSumPolys[j]) != claims.Degree(j) {
			return fmt.Errorf("malformed proof") //Malformed proof
		}
		copy(gJ[1:], proof.PartialSumPolys[j])
		gJ[0] = api.Sub(gJR, proof.PartialSumPolys[j][0]) // Requirement that gⱼ(0) + gⱼ(1) = gⱼ₋₁(r)
		// gJ is ready

		//Prepare for the next iteration
		r[j] = transcript.Next(proof.PartialSumPolys[j])

		gJR = InterpolateOnRange(api, r[j], gJ[:(claims.Degree(j)+1)]...)
	}

	return claims.VerifyFinalEval(api, r, combinationCoeff, gJR, proof.FinalEvalProof)
}