package toolchain

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// A Compiler can compile one C/C++ source to an executable.
type Compiler interface {
	// Compile compiles the given input using a C++ compiler to an executable.
	// It parses the given options to the format required by the underlying compiler
	// and then outputs the compiled source file as an executable,
	// which is written to the provided writer. The compile options may be nil.
	Compile(ctx context.Context, input io.Reader, output io.Writer, options *CompileOptions) error
	// Info returns some information about the compiler.
	Info() CompilerInfo
}

// CompileOptions customizes the compilation process in a compiler-agnostic way.
// Each option is translated to the compiler-specific flags.
type CompileOptions struct {
	// IncludePaths where additional headers should be found.
	IncludePaths []string
	// LibraryPaths where libraries required for linking should be found.
	LibraryPaths []string
	// Libraries to link the executable to.
	Libraries []string
	// LanguageStandard specifies the language standard used to compile the source.
	// Make sure the source code provided complies to the standard and that the compiler
	// supports the standard.
	LanguageStandard CompileLanguageStandard
	// OptimizationLevel specifies the level of optimization applied to the executable.
	OptimizationLevel CompileOptimizationLevel
	// Defines specifies a list of macros that should be defined.
	Defines []string
	// Undefs specifies a list of macros that should be undefined.
	Undefs []string
	// Flags can be used to specifiy other compiler options that are not available
	// in CompileOptions. These flags are not translated, so compilers may not be
	// able to be used interchangeably when this option is used. They also override
	// any flags set by the fields from CompileOptions.
	Flags Flags
}

// CompileOptimizationLevel values are used to specify the optimization level used by the compiler.
// These levels do not toggle only the optimization flags of the compiler. For convenience, they
// might also toggle other flags - see the documentation for each value.
type CompileOptimizationLevel int

const (
	// CompileOptimizationNone means no optimizaiton. It is equivalent to -O0 (GCC, Clang) or /Od (MSVC).
	CompileOptimizationNone CompileOptimizationLevel = iota
	// CompileOptimizationModerate provides good optimization without great impact over the compilation time.
	// It is equivalent to -O1 (GCC, Clang) or /O1 (MSVC).
	CompileOptimizationModerate
	// CompileOptimizationAggressive provides best optimization for speed, sacrificing compilation time.
	// It is equivalent to -O2 (GCC, Clang) or /O2 (MSCV).
	CompileOptimizationAggressive
	// CompileOptimizationDebug provides the right combination of flags to the underlying compiler
	// for the best debugging experience. For example, on GCC it sets the -ggdb and -Og flags,
	// on Clang -g and -O0, and on MSVC /Od and /Z7
	CompileOptimizationDebug
)

// CompileLanguageStandard values are used to specify the desired C/C++ standard used by the compiler.
// If the compiler has extensions for the specified standard, they will not be used, aiding in portability.
// If the compiler does not support the specified standard, compilation will fail.
type CompileLanguageStandard int

const (
	// CompileLanguageStandardDefault is the default language standard used by the compiler.
	// In other words, no standard flag is passed.
	CompileLanguageStandardDefault CompileLanguageStandard = iota
	CompileLanguageStandardC90
	CompileLanguageStandardC99
	CompileLanguageStandardC11
	CompileLanguageStandardC17
	CompileLanguageStandardCPP98
	CompileLanguageStandardCPP03
	CompileLanguageStandardCPP11
	CompileLanguageStandardCPP14
	CompileLanguageStandardCPP17
	CompileLanguageStandardCPP20
)

// CompilerInfo holds some information about the underlying compiler.
type CompilerInfo struct {
	// Name of the compiler.
	Name string
	// Path of the compiler's executable.
	Path string
	// Version number of the compiler.
	Version string
	// RecommendedDebugger to be used with executables outputted by this compiler.
	// If non-empty, the name can be used to instantiate a Debugger instance, if
	// the debugger implementation with the given name is present.
	RecommendedDebugger string
}

// NewCompiler looks up the compiler's executable with the given name on the host
// and initializes a Compiler instance that uses that executable.
func NewCompiler(name string) (Compiler, error) {
	compilersMutex.RLock()
	constructor := compilers[name]
	compilersMutex.RUnlock()

	if constructor == nil {
		return nil, fmt.Errorf("toolchain: missing compiler %q, forgotten import?", name)
	}

	compiler, err := constructor(name)
	if err != nil {
		return nil, fmt.Errorf("toolchain: failed to initialize compiler %q: %w", name, err)
	}

	return compiler, nil
}

// DetectCompilers returns all the supported (imported) compiler toolchains available on the host system.
func DetectCompilers() []Compiler {
	compilersMutex.RLock()
	defer compilersMutex.RUnlock()

	return detectCompilers()
}

func detectCompilers() []Compiler {
	var found []Compiler

	for _, name := range compilersNames {
		compiler, err := compilers[name](name)
		if err != nil {
			continue
		}

		found = append(found, compiler)
	}

	return found
}

// UsePreferredCompiler tries to initialize the compiler specified by the CXX environment variable.
// If CXX is empty, it falls back to the first value returned by DetectCompilers. If no compiler
// was detected, it returns an error.
func UsePreferredCompiler() (Compiler, error) {
	compilersMutex.RLock()
	defer compilersMutex.RUnlock()

	name := os.Getenv("CXX")
	if name != "" {
		for _, registeredName := range compilersNames {
			if strings.Contains(name, registeredName) {
				compiler, err := compilers[registeredName](name)
				if err != nil {
					break
				}

				return compiler, nil
			}
		}
	}

	compilers := detectCompilers()
	if len(compilers) == 0 {
		return nil, fmt.Errorf("toolchain: no compilers registered, forgotten imports?")
	}

	return compilers[0], nil
}

// CompilerConstructor is a function that constructs a Compiler from an executable.
// It takes either a path to the executable or the executable's name as an argument.
type CompilerConstructor func(pathOrExecutableName string) (Compiler, error)

var (
	compilers      = map[string]CompilerConstructor{}
	compilersNames []string // provide ordered iteration for the map
	compilersMutex sync.RWMutex
)

// RegisterCompiler adds a custom Compiler implementation for usage.
// If an implementation with the same name already exists or the provided
// constructor is nil, this function panics. If the name has path separators
// or path list separators, this function panics.
//
// The provided name may be used by the constructor to look up the path of the compiler's executable.
func RegisterCompiler(name string, constructor CompilerConstructor) {
	compilersMutex.Lock()
	defer compilersMutex.Unlock()

	if !isValidImplementationName(name) {
		panic(fmt.Sprintf("toolchain: compiler name %q has invalid characters", name))
	}

	if compilers[name] != nil {
		panic(fmt.Sprintf("toolchain: compiler %q is already registered", name))
	}

	if constructor == nil {
		panic(fmt.Sprintf("toolchain: constructor provided for compiler %q is nil", name))
	}

	compilers[name] = constructor
	compilersNames = append(compilersNames, name)
}
