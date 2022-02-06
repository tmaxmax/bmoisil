package gcc

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tmaxmax/bmoisil/pkg/toolchain"
)

func parseVersion(stdout []byte) string {
	versionLineEnd := bytes.IndexAny(stdout, "\r\n")
	if versionLineEnd == -1 {
		versionLineEnd = len(stdout)
	}
	versionLine := stdout[:versionLineEnd]
	return string(versionLine[bytes.LastIndexByte(versionLine, ' ')+1:])
}

// Debugger is a GDB debugger.
type Debugger struct {
	info toolchain.DebuggerInfo
}

// NewDebugger creates a gdb debugger instance. It looks up an executable using the provided name
// or uses the executable at the given path, if a path is specified.
func NewDebugger(nameOrPath string) (*Debugger, error) {
	cmd := execCommandContext(context.Background(), nameOrPath, "--version")
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gcc: failed to initialize debugger: %w", err)
	}

	info := toolchain.DebuggerInfo{
		Name:                debuggerName,
		Path:                cmd.Path,
		Version:             parseVersion(stdout),
		RecommendedCompiler: compilerName,
	}

	return &Debugger{info: info}, nil
}

func (d *Debugger) Debug(ctx context.Context, executablePath string, streams *toolchain.DebuggerStreams) error {
	cmd := execCommandContext(ctx, d.info.Path, executablePath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = toolchain.GetDebuggerStreams(streams)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcc: failed to debug: %w", err)
	}

	return nil
}

func (d *Debugger) Info() toolchain.DebuggerInfo {
	return d.info
}
