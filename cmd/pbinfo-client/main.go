package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gocolly/colly/v2/debug"
	"github.com/tmaxmax/bmoisil/pkg/pbinfo"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var id int
	var showTestCases bool
	var useDebugger bool
	var sizeLimit int

	flag.IntVar(&id, "id", 0, "The ID of the PbInfo problem to retrieve")
	flag.BoolVar(&showTestCases, "show-test-cases", false, "Whether to output the test cases or not")
	flag.BoolVar(&useDebugger, "debug", false, "Use a debugger for the web scraper")
	flag.IntVar(&sizeLimit, "size-limit", 1e3, "The maximum test case content size to show")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := &pbinfo.Client{}
	if useDebugger {
		client.CollectorDebugger = &debug.LogDebugger{}
	}

	g, gctx := errgroup.WithContext(ctx)

	var testCases []pbinfo.TestCase
	if showTestCases {
		g.Go(func() error {
			cases, err := client.GetProblemTestCases(gctx, id)
			testCases = cases
			return err
		})
	}

	g.Go(func() error {
		problem, err := client.FindProblemByID(gctx, id)
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(problem)
	})

	if err := g.Wait(); err != nil {
		if errors.Is(err, ctx.Err()) {
			return nil
		}

		return err
	}

	for i, t := range testCases {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Printf("\nTest case %d", i+1)
		if t.IsExample {
			fmt.Print(" (example)")
		}
		fmt.Println(":")
		if t.Score != 0 {
			fmt.Printf("Score: %d\n", t.Score)
		}

		input := normalizeTestCaseContent(t.Input, sizeLimit)
		output := normalizeTestCaseContent(t.Output, sizeLimit)

		fmt.Printf("Input: %s\nOutput: %s\n", input, output)
	}

	return nil
}

func normalizeTestCaseContent(c []byte, sizeLimit int) string {
	if len(c) <= sizeLimit {
		return string(c)
	}

	i := bytes.LastIndexByte(c[:sizeLimit], ' ')

	return string(c[:i]) + "... [truncated]"
}
