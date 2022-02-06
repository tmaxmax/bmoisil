/*
Package gcc provides a compiler and debugger implementation
that uses the installed GCC toolchain on the host system.

It registers the g++ compiler and the gdb debugger.
*/
package gcc

import (
	"os/exec"

	"github.com/tmaxmax/bmoisil/pkg/toolchain"
)

const (
	compilerName = "g++"
	debuggerName = "gdb"
)

var execCommandContext = exec.CommandContext

func init() {
	toolchain.RegisterCompiler(compilerName, func(pathOrExecutableName string) (toolchain.Compiler, error) {
		return NewCompiler(pathOrExecutableName)
	})
	toolchain.RegisterDebugger(debuggerName, func(pathOrExecutableName string) (toolchain.Debugger, error) {
		return NewDebugger(pathOrExecutableName)
	})
}
