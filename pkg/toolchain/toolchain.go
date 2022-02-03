/*
Package toolchain provides a set of utilities to manage and use
C/C++ compilers, debuggers and memory checkers. It abstracts their
usage under convenient interfaces so tools can be interchangeably
utilized.
*/
package toolchain

import (
	"os"
	"strings"
)

func isValidImplementationName(name string) bool {
	return !strings.ContainsAny(name, string([]rune{os.PathSeparator, os.PathListSeparator}))
}
