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
}

//go:embed Comment.go
var commentGo string

// FIXME: Sync the test code with the main code so that the comments
//  are preserved in both scenarios.
