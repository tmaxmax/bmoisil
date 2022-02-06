package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/tmaxmax/bmoisil/pkg/toolchain"
	_ "github.com/tmaxmax/bmoisil/pkg/toolchain/gcc"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	compiler, err := toolchain.UsePreferredCompiler()
	if err != nil {
		return err
	}

	info := compiler.Info()

	fmt.Printf("Compiler: %s\nPath: %s\nVersion: %s\n\n", info.Name, info.Path, info.Version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	filepath := "./compiled"
	if runtime.GOOS == "windows" {
		filepath += ".exe"
	}

	if err := compiler.Compile(ctx, os.Stdin, filepath, &toolchain.CompileOptions{
		OptimizationLevel: toolchain.CompileOptimizationAggressive,
		LanguageStandard:  toolchain.CompileLanguageStandardCPP11,
	}); err != nil {
		return err
	}
	defer os.Remove(filepath)

	cmd := exec.CommandContext(ctx, filepath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return err
	}

	debugger, err := toolchain.NewDebugger(info.RecommendedDebugger)
	if err != nil {
		return err
	}

	dinfo := debugger.Info()
	fmt.Printf("Debugger: %s\nPath: %s\nVersion: %s\n\n", dinfo.Name, dinfo.Path, dinfo.Version)

	return debugger.Debug(ctx, filepath, nil)
}
