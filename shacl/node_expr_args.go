package shacl

// The arguments of a list parameter function (sparql:*, shnex:conformsToShape
// and the custom sh:ListParameterExpressionFunction ones) are evaluated before
// the function is applied, each to a single node:
//
//   - one output node is the value of the argument;
//   - more than one output node, or an evaluation failure, makes the argument
//     failed (a SPARQL error for the SPARQL functions);
//   - no output node leaves the argument unbound.
//
// SHACL 1.2 Node Expressions, "List Parameter Functions".

// argument is the value of one evaluated argument of a list parameter function.
type argument struct {
	node  Term  // the single output node; set when bound
	bound bool  // exactly one output node
	err   error // the argument failed: a failure, or more than one output node
}

// evalListArguments evaluates every argument, in order, with the scope and
// focus node of the calling expression.
func evalListArguments(ctx *nodeExprContext, args []NodeExpr) []argument {
	out := make([]argument, len(args))
	for i, a := range args {
		nodes, err := evalNodeExpr(ctx, a)
		switch {
		case err != nil:
			out[i].err = err
		case len(nodes) > 1:
			out[i].err = failf("argument %d produced %d output nodes, one is required", i+1, len(nodes))
		case len(nodes) == 1:
			out[i].node, out[i].bound = nodes[0], true
		}
	}
	return out
}

// firstArgumentFailure returns the failure of the first failed argument. A
// failed argument takes precedence over an unbound one, whatever their order.
func firstArgumentFailure(args []argument) error {
	for _, a := range args {
		if a.err != nil {
			return a.err
		}
	}
	return nil
}

// allBound reports whether every argument has a value.
func allBound(args []argument) bool {
	for _, a := range args {
		if !a.bound {
			return false
		}
	}
	return true
}
