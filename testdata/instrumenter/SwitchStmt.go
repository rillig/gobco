package instrumenter

// https://go.dev/ref/spec#Switch_statements

// TODO: Add systematic tests.

func switchStmt(expr int, cond bool, s string) {

	// In a switch statement without tag, all
	// expressions in the case clauses have type bool,
	// therefore they are instrumented.
	switch {

	case expr == 5:

	case cond:
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

	// In a switch statement with an init assignment, the tag expression is
	// appended to that assignment, preserving the order of evaluation.
	//
	// The init operator is changed from = to :=. This does not declare new
	// variables for the existing variables.
	// See https://golang.org/ref/spec#ShortVarDecl, keyword redeclare.
	switch s = "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}
	// Same for a short declaration in the initialization.
	switch s := "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}

	// No matter whether there is an init statement or not, if the tag
	// expression is empty, the comparisons use the simple form and are not
	// compared to an explicit "true".
	switch s := "prefix" + s; {
	case s == "one":
	}

	// If the left-hand side and the right-hand side of the assignment don't
	// agree in the number of elements, it is not possible to add the gobco
	// variable to that list. The assignment to the gobco variable is
	// always separate from any other initialization statement.
	switch a, b := (func() (string, string) { return "a", "b" })(); cond {
	case true:
		a += b
		b += a
	}

	// Switch statements that contain a tag expression and an
	// initialization statement are wrapped in an outer no-op switch
	// statement, to preserve the scope in which the initialization and
	// the tag expression are evaluated.
	ch := make(chan<- int, 1)
	switch ch <- 3; expr {
	case 5:
	}
}
