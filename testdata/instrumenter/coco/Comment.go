// Build constraint comments must be at the top of the file.
//
//go:build linux || !linux
// +build linux !linux

package instrumenter

import _ "embed"

// https://go.dev/ref/spec#Comments
// https://pkg.go.dev/cmd/go#hdr-Build_constraints

// comment covers the instrumentation of [ast.Comment], which has no
// expression fields.
//
// Comments that influence the build process must be preserved during
// instrumentation. Examples for such comments are '//go:build' and
// '//go:embed'.
func comment() {
	// TODO: Try to move the 'go:embed' comment away from its variable
	//  declaration, so that it becomes ignored.

	// When gobco instruments a type switch statement, it moves the type
	// expressions further up in the code but keeps the position
	// information from the original type expression.
	switch interface{}(nil).(type) {
	case int:
		// begin int
		_ = 1
		// end int
	case [][][][]int:
		// begin int-4D
		_ = 1
		// end int-4D
	case [][][]int:
		// begin int-3D
		_ = 1
		// end int-3D
	case [][]int:
		// begin int-2D
		_ = 1
		// end int-2D
	case []int:
		// begin int-1D
		_ = 1
		// end int-1D
	}
	// comment after switch
}

//go:embed Comment.go
var commentGo string
