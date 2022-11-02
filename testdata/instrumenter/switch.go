package instrumenter

func switchStmt(expr int, cond bool, s string) {

	// In a switch statement without tag, all
	// expressions in the case clauses have type bool,
	// therefore they are instrumented.
	switch {

	case expr == 5:

	case cond: // TODO: instrument this condition
		// The expression 'cond' has type bool, even without looking at
		// its variable definition, as it is compared to the implicit 'true'
		// from the 'switch' tag.
	}

	// In switch statements with a tag but no initialization statement,
	// the value of the tag expression can be evaluated in the
	// initialization statement, without wrapping the whole switch
	// statement in another switch statement.
	switch s {
	case "one",
		"two",
		"three":
	}

	// In switch statements with a tag expression, the expression is
	// evaluated exactly once and then compared to each expression from
	// the case clauses.
	switch s + "suffix" {
	case "one",
		"two",
		"" + s:
	}
}
