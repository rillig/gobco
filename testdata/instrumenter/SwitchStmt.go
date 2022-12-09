package instrumenter

// https://go.dev/ref/spec#Switch_statements

// TODO: Add systematic tests.

// switchStmt covers the instrumentation of [ast.SwitchStmt], which has the
// expression field Tag.
func switchStmt(expr int, cond bool, s string) {

	// In switch statements without tag, the tag is implicitly 'true',
	// therefore all expressions in the case clauses must have type bool,
	// therefore they are instrumented.
	_ = "no init; no tag"
	switch {
	case expr == 5:
	case cond:
	}

	// In switch statements with a tag but no initialization statement,
	// the value of the tag expression can be evaluated in the
	// initialization statement, without wrapping the whole switch
	// statement in another switch statement.
	//
	// In this case, the tag is a plain identifier, therefore it isn't even
	// necessary to invent a temporary variable for the tag. It is done
	// nevertheless, to keep the instrumenting code simple.
	_ = "no init; tag is an identifier"
	switch s {
	case "one",
		"two",
		"three":
	}

	// In switch statements with a tag expression, the expression is
	// evaluated exactly once and then compared to each expression from
	// the case clauses.
	_ = "no init; tag is a complex expression"
	switch s + "suffix" {
	case "one",
		"two",
		"" + s:
	}

	// In a switch statement with an init assignment, the initialization
	// happens before evaluating the tag expression.
	//
	// In a previous version of gobco, the temporary variable for the tag
	// expression was added to the assignment list, which was wrong because
	// it changed the order of evaluation.
	_ = "init overwrites variable; tag uses the overwritten variable"
	switch s = "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}

	// Same for a short declaration in the initialization.
	_ = "init defines new variable; tag uses the new variable"
	switch s := "prefix" + s; s + "suffix" {
	case "prefix.a.suffix":
	}

	// No matter whether there is an init statement or not, if the tag
	// expression is empty, the comparisons use the simple form and are not
	// compared to an explicit "true".
	_ = "init, but no tag"
	switch s := "prefix" + s; {
	case s == "one":
	case cond:
	}

	// The statements from the initialization are simply copied, there is no
	// need to handle assignments of multi-valued function calls differently.
	//
	// In a previous version, gobco tried to add its temporary tag variable
	// to the assignment statement, but that was wrong because it changed
	// the order of evaluation.
	_ = "init with multi-valued function call"
	switch a, b := (func() (string, string) { return "a", "b" })(); cond {
	case true:
		a += b
		b += a
	}

	// Switch statements that contain a tag expression and an
	// initialization statement are wrapped in an outer no-op switch
	// statement, to preserve the scope in which the initialization and
	// the tag expression are evaluated.
	_ = "init with non-assignment"
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

	// In a switch statement without tag expression, ensure that complex
	// conditions in the case clauses are not instrumented redundantly.
	switch a, b := cond, !cond; {
	case (a && b):
	case (a || b):
	}
}
