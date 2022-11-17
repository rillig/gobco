package instrumenter

import (
	"reflect"
)

// https://go.dev/ref/spec#Type_switches

// typeSwitchStmt covers the instrumentation of [ast.TypeSwitchStmt], which
// has no expression fields.
//
// A type switch statement contains implicit comparisons that need to be
// instrumented.
func typeSwitchStmt(tag interface{}) string {

	// The type switch guard can be a simple expression.
	switch tag.(type) {
	}

	// The type switch guard can be a short variable declaration for a
	// single variable, in which case each branch gets its own declared
	// variable, with the proper type.
	switch v := tag.(type) {
	default:
		_ = v
	}

	// A type switch statement may have an initialization statement that is
	// evaluated in a nested scope. The type switch tag can be a short
	// variable definition, which has another, nested scope, in each of the
	// case clauses.
	switch tag := tag; tag := tag.(type) {
	default:
		_ = tag
	}

	// Type expressions may be parenthesized:
	switch tag.(type) {
	case (int):
		return "parenthesized " + reflect.TypeOf(tag).Name()
	}

	switch tag.(type) {
	case nil:
		return "nil"
	}

	// In case clauses with a single type, the variable has that type.
	// In all other cases, the variable has the type of the guard expression.
	// The type identifier 'nil' matches a nil interface value.
	switch v := tag.(type) {
	case uint:
		_ = v + uint(0)
		return "uint " + reflect.TypeOf(v).Name()
	case uint8, uint16:
		return "any " + reflect.TypeOf(v).Name()
	case nil:
		// unreachable
		return "nil " + reflect.TypeOf(v).Name()
	}

	// TODO: Test type parameters and generic types.
	return "end"
}

func typeSwitchStmtScopes(value interface{}) string {

	// Gobco does not instrument type switch statements, as rewriting them
	// requires more code changes than for ordinary switch statements. It's
	// pretty straightforward though, see typeSwitchStmtScopesInstrumented
	// below for a possible approach.

	switch _ = 123 > 0; v := value.(type) {

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
	// are instrumented.
	//
	// As with an ordinary switch statement, the implicit scopes need to be
	// modeled correctly:
	//
	// The outer scope is for the initialization statement and the tag
	// expression (tmp0).
	//
	// The inner scope is per case clause and contains the expression from
	// the switch statement, converted to the proper type.

	switch _ = 123 > 0; interface{}(0).(type) {
	default:
		tmp0 := value
		_, tmp1 := tmp0.(int)
		_, tmp2 := tmp0.(uint)
		_, tmp3 := tmp0.(string)
		_, tmp4 := tmp0.(struct{})
		_, tmp5 := tmp0.(uint8)
		tmp6 := tmp0 == nil

		switch {

		case tmp1, tmp2:
			v := tmp0
			_ = v
			return "integer " + reflect.TypeOf(v).String()

		case tmp3:
			v := tmp0.(string)
			_ = v
			return "string " + reflect.TypeOf(v).String()

		case tmp4:
			v := tmp0.(struct{})
			_ = v
			return "struct{} " + reflect.TypeOf(v).String()

		case tmp5:
			v := tmp0.(uint8)
			_ = v
			return "byte"

		case tmp6:
			v := tmp0
			_ = v
			return "nil"

		default:
			v := tmp0
			return "other " + reflect.TypeOf(v).String()
		}
	}
}
