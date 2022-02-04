package gcc

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/tmaxmax/bmoisil/pkg/toolchain"
)

var standardsRepresentation = map[toolchain.CompileLanguageStandard]string{
	toolchain.CompileLanguageStandardC90:   "c90",
	toolchain.CompileLanguageStandardC99:   "c99",
	toolchain.CompileLanguageStandardC11:   "c11",
	toolchain.CompileLanguageStandardC17:   "c17",
	toolchain.CompileLanguageStandardCPP98: "c++98",
	toolchain.CompileLanguageStandardCPP03: "c++03",
	toolchain.CompileLanguageStandardCPP11: "c++11",
	toolchain.CompileLanguageStandardCPP14: "c++14",
	toolchain.CompileLanguageStandardCPP17: "c++17",
	toolchain.CompileLanguageStandardCPP20: "c++20",
}

func addLanguageStandardFlag(flags toolchain.Flags, standard toolchain.CompileLanguageStandard) {
	standardRepr := standardsRepresentation[standard]
	if standardRepr == "" {
		return
	}

	flags.Set("std", standardRepr)
}

func addOptimizationFlags(flags toolchain.Flags, optimization toolchain.CompileOptimizationLevel) {
	switch optimization {
	case toolchain.CompileOptimizationNone:
		flags.Set("O", "0")
	case toolchain.CompileOptimizationModerate:
		flags.Set("O", "1")
	case toolchain.CompileOptimizationAggressive:
		flags.Set("O", "2")
	case toolchain.CompileOptimizationDebug:
		flags.Set("O", "g")
		flags.Toggle("ggdb")
	}
}

func addSourceKindFlag(flags toolchain.Flags, kind toolchain.SourceFileKind) {
	if kind == toolchain.SourceFileKindC {
		flags.Set("x", "c")
	} else {
		flags.Set("x", "c++")
	}
}

func parseOptions(outputPath string, opts *toolchain.CompileOptions) []string {
	flags := make(toolchain.Flags)
	flags.Set("o", outputPath)

	if opts == nil {
		flags.Set("x", "c++")
		return parseFlags(flags)
	}

	flags.Set("D", opts.Defines...)
	flags.Set("U", opts.Undefs...)
	flags.Set("L", opts.LibraryPaths...)
	flags.Set("l", opts.Libraries...)
	flags.Set("I", opts.IncludePaths...)
	addLanguageStandardFlag(flags, opts.LanguageStandard)
	addOptimizationFlags(flags, opts.OptimizationLevel)
	addSourceKindFlag(flags, opts.SourceFileKind)
	flags.Merge(opts.Flags)

	return parseFlags(flags)
}

func parseFlags(flags toolchain.Flags) []string {
	const flagStart = "-"
	var out []string

	flags.Range(func(flag string, values []string, isToggle bool) {
		if isToggle {
			out = append(out, flagStart+flag)
			return
		}

		// TODO: quote value if necessary?

		for _, value := range values {
			switch flag {
			case "O", "D", "L", "l", "I":
				out = append(out, flagStart+flag+value)
			case "std":
				out = append(out, flagStart+flag+"="+value)
			default:
				out = append(out, flagStart+flag)
				out = append(out, value)
			}
		}
	})

	return out
}

type Compiler struct {
	info toolchain.CompilerInfo
}

var _ toolchain.Compiler = (*Compiler)(nil)

func NewCompiler(pathOrExec string) (*Compiler, error) {
	cmd := execCommandContext(context.Background(), pathOrExec, "-dumpversion")
	version, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gcc: failed to initialize compiler: %w", err)
	}

	info := toolchain.CompilerInfo{
		Name:                compilerName,
		Path:                cmd.Path,
		Version:             string(bytes.TrimSpace(version)),
		RecommendedDebugger: debuggerName,
	}

	return &Compiler{info: info}, nil
}

func (c *Compiler) Compile(ctx context.Context, input io.Reader, outputPath string, opts *toolchain.CompileOptions) error {
	args := append(parseOptions(outputPath, opts), "-")
	cmd := execCommandContext(ctx, c.info.Path, args...)
	cmd.Stdin = input

	if err := cmd.Run(); err != nil {
		// TODO: Actual compilation error message - custom error type
		return fmt.Errorf("gcc: failed to compile: %w", err)
	}

	return nil
}

func (c *Compiler) Info() toolchain.CompilerInfo {
	return c.info
}
