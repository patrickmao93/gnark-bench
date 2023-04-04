package constraint

import (
	"github.com/consensys/gnark/constraint/solver"
)

// HintMapping mark a list of output variables to be computed using provided hint and inputs.
type HintMapping struct {
	HintID  solver.HintID      // Hint function id
	Inputs  []LinearExpression // Terms to inject in the hint function
	Outputs []int              // IDs of wires the hint outputs map to
}

// WireIterator implements constraint.Iterable
func (h *HintMapping) WireIterator() func() int {
	curr := 0
	return func() int {
		if curr < len(h.Outputs) {
			curr++
			return h.Outputs[curr-1]
		}
		n := len(h.Outputs)
		// TODO @gbotrel revisit that looks terrible.
		for i := 0; i < len(h.Inputs); i++ {
			n += len(h.Inputs[i])
			for curr < n {
				curr++
				idx := curr - 1 - n + len(h.Inputs[i])
				term := h.Inputs[i][idx]
				if term.IsConstant() {
					continue
				}
				return int(term.VID)
			}
		}
		return -1
	}
}
