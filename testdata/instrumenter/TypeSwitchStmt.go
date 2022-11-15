package instrumenter

import (
	"reflect"
)

// https://go.dev/ref/spec#Type_switches

// TODO: Add systematic tests.

// typeSwitchStmt covers the instrumentation of [ast.TypeSwitchStmt], which
// has no expression fields.
//
// A type switch statement contains implicit comparisons that need to be
// instrumented.
func typeSwitchStmt() {
}

func typeSwitchStmtScopes(value interface{}) string {

	// Gobco does not instrument type switch statements, as rewriting them
	// requires more code changes than for ordinary switch statements. It's
	// pretty straightforward though, see typeSwitchStmtScopesInstrumented
	// below for a possible approach.

	switch v := value.(type) {

	case int, uint:
		// In a clause that lists multiple types, the expression 'v' has the
		// type of the switch tag, in this case 'interface{}'.
		return "integer " + reflect.TypeOf(v).String()

	case string:
		// In a clause that lists a single type, the expression 'v' has the
		// type from the case clause.
		return "string " + reflect.TypeOf(v).String()

	case struct{}:
		return "struct{} " + reflect.TypeOf(v).String()

	case uint8:
		// The variable 'v' may be unused in some of the case clauses.
		return "byte"

	case nil:
		return "nil"

	default:
		return "other " + reflect.TypeOf(v).String()
	}
}

func typeSwitchStmtScopesInstrumented(value interface{}) string {

	// This is how the type switch statement from typeSwitchStmtScopes
	// could be instrumented.
	//
	// As with an ordinary switch statement, the implicit scopes need to be
	// modeled correctly:
	//
	// The outer scope is for the initialization statement and the tag
	// expression (tmp0).
	//
	// The inner scope is per case clause and contains the expression from
	// the switch statement, converted to the proper type.

	switch tmp0 := value; {

	case func() bool { _, ok := tmp0.(int); return ok }(),
		func() bool { _, ok := tmp0.(uint); return ok }():
		v := tmp0
		_ = v
		return "integer " + reflect.TypeOf(v).String()

	case func() bool { _, ok := tmp0.(string); return ok }():
		v := tmp0.(string)
		_ = v
		return "string " + reflect.TypeOf(v).String()

	case func() bool { _, ok := tmp0.(struct{}); return ok }():
		v := tmp0.(struct{})
		_ = v
		return "struct{} " + reflect.TypeOf(v).String()

	case func() bool { _, ok := tmp0.(uint8); return ok }():
		v := tmp0.(uint8)
		_ = v
		return "byte"

	case tmp0 == nil:
		return "nil"

	default:
		v := tmp0
		return "other " + reflect.TypeOf(v).String()
	}
}
