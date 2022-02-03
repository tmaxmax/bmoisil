package toolchain

import (
	"context"
	"fmt"
	"sync"
)

// A Debugger is used to debug compiled executables.
type Debugger interface {
	// Debug runs the debugger for the given executable.
	Debug(ctx context.Context, executablePath string) error
	// Info returns some information about the debugger.
	Info() DebuggerInfo
}

// DebuggerInfo holds some information about the underlying debugger.
type DebuggerInfo struct {
	// Name of the debugger.
	Name string
	// Path of the debugger's executable.
	Path string
	// Version number of the debugger.
	Version string
	// RecommendedCompiler to generate executables for this debugger.
	// If non-empty, the name can be used to instantiate a Debugger instance,
	// if the compiler implementation with the given name is present.
	RecommendedCompiler string
}

// NewDebugger looks up the debugger's executable with the given name on the host
// and initializes a Debugger instance that uses that executable.
func NewDebugger(name string) (Debugger, error) {
	debuggersMutex.RLock()
	constructor := debuggers[name]
	debuggersMutex.RUnlock()

	if constructor == nil {
		return nil, fmt.Errorf("toolchain: missing debugger %q, forgotten import?", name)
	}

	debugger, err := constructor(name)
	if err != nil {
		return nil, fmt.Errorf("toolchain: failed to initialize debugger %q: %w", name, err)
	}

	return debugger, nil
}

// DetectDebuggers returns all the supported (imported) debuggers available on the host system.
func DetectDebuggers() []Debugger {
	debuggersMutex.RLock()
	defer debuggersMutex.RUnlock()

	var found []Debugger

	for _, name := range compilersNames {
		debugger, err := debuggers[name](name)
		if err != nil {
			continue
		}

		found = append(found, debugger)
	}

	return found
}

// DebuggerConstructor is a function that constructs a Debugger from an executable.
// It takes either the path to the executable or the executable's name as an argument.
type DebuggerConstructor func(pathOrExecutableName string) (Debugger, error)

var (
	debuggers      = map[string]DebuggerConstructor{}
	debuggersNames []string
	debuggersMutex sync.RWMutex
)

// RegisterDebugger adds a custom Debugger implementation for usage.
// If an implementation with the same name already exists or the provided
// constructor is nil, this function panics. If the name has path separators
// or path list separators, this function panics.
//
// The provided name may be used by the constructor to look up the path of the debugger's executable.
func RegisterDebugger(name string, constructor DebuggerConstructor) {
	debuggersMutex.Lock()
	defer debuggersMutex.Unlock()

	if !isValidImplementationName(name) {
		panic(fmt.Sprintf("toolchain: debugger name %q has invalid characters", name))
	}

	if debuggers[name] != nil {
		panic(fmt.Sprintf("toolchain: debugger %q is already registered", name))
	}

	if constructor == nil {
		panic(fmt.Sprintf("toolchain: debugger provided for compiler %q is nil", name))
	}

	debuggers[name] = constructor
	debuggersNames = append(debuggersNames, name)
}
