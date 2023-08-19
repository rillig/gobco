package instrumenter

// https://go.dev/ref/spec#Switch_statements

// TODO: Add systematic tests.

// switchStmt covers the instrumentation of [ast.SwitchStmt], which has the
// expression field Tag, plus several implicit comparisons.
//
// In condition coverage mode, the Tag expression is instrumented.
func switchStmt(expr int, cond bool, s string) {

	// In switch statements without tag, the tag is implicitly 'true',
	// therefore all expressions in the case clauses must have type bool,
	// therefore they are instrumented.
	switch {
	case expr == 5:
	case cond:
	}

	// No matter whether there is an init statement or not, if the tag
	// expression is empty, the comparisons use the simple form and are not
	// compared to an explicit "true".
	switch s := "prefix" + s; {
	case s == "one":
	case cond:
	}

	// In a switch statement without tag expression, ensure that complex
	// conditions in the case clauses are not instrumented redundantly.
	switch a, b := cond, !cond; {
	case (a && b):
	case (a || b):
	}

	// No initialization, the tag is a plain identifier.
	// The instrumented code could directly compare the tag with the
	// expressions from the case clauses.
	// It doesn't do so, to keep the instrumenting code simple.
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

	// In a switch statement with an init statement, the init statement
	// happens before evaluating the tag expression.
	switch s = "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}

	// In a switch statement with an init variable definition, the
	// variable is defined in a separate scope, and the initialization
	// statement happens before evaluating the tag expression.
	switch s := "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}

	// The statements from the initialization are simply copied, there is no
	// need to handle assignments of multi-valued function calls differently.
	switch a, b := (func() (string, string) { return "a", "b" })(); cond {
	case true:
		a += b
		b += a
	}

	// Switch statements that contain a tag expression and an
	// initialization statement are wrapped in an outer block.
	// In this case, the block would not be necessary since the
	// gobco variable name does not clash with the code that is
	// instrumented.
	ch := make(chan<- int, 1)
	switch ch <- 3; expr {
	case 5:
	}

	// In the case clauses, there may be complex conditions.
	// In the case of '!a', the condition 'a' is already instrumented,
	// so instrumenting '!a' seems redundant at first.
	// The crucial point is that it's not the value of 'a' alone that
	// decides which branch is taken, but instead 'cond == a'.
	switch a, b := cond, !cond; cond {
	case a:
	case !a:
	case (!a):
	case a && b:
	case a && !b:
	case a || b:
	case !a || b:
	case a == b:
	case a != b:
	}

	// In a switch statement, the tag expression may be unused.
	switch 1 > 0 {
	}
}
