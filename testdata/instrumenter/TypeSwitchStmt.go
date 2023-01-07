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
func typeSwitchStmt(tag interface{}, value interface{}) string {

	// An empty type switch statement doesn't need to be instrumented.
	switch tag.(type) {
	}

	// The type switch guard can be a simple expression.
	switch tag.(type) {
	default:
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

	// Nil may be parenthesized:
	switch tag.(type) {
	case (nil):
		return "parenthesized nil"
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

func typeSwitchStmtMixed(value interface{}) {
	// XXX: The instrumentation does not happen strictly in
	//  declaration order:
	//  All types from the TypeSwitchStmt are instrumented
	//  in a first pass.
	//  All other expressions are instrumented in a second pass.
	switch value.(type) {
	case int:
		_ = true && false
	case uint:
		_ = false || true
	}
}
