package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmaxmax/bmoisil/pkg/pbinfo"
)

func main() {
	var id int
	var showTestCases bool
	var sizeLimit int

	flag.IntVar(&id, "id", 0, "The ID of the PbInfo problem to retrieve")
	flag.BoolVar(&showTestCases, "show-test-cases", false, "Whether to output the test cases or not")
	flag.IntVar(&sizeLimit, "size-limit", 1e3, "The maximum test case content size to show")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := &pbinfo.Client{}

	problem, err := client.FindProblemByID(ctx, id)
	if err != nil {
		log.Fatalln(err)
	}

	var testCases []pbinfo.TestCase
	if showTestCases {
		testCases, err = client.GetProblemTestCases(ctx, id)
		if err != nil {
			log.Fatalln(err)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(problem); err != nil {
		log.Fatalln(err)
	}

	for i, t := range testCases {
		select {
		case <-ctx.Done():
			return
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
}

func normalizeTestCaseContent(c []byte, sizeLimit int) string {
	if len(c) <= sizeLimit {
		return string(c)
	}

	i := bytes.LastIndexByte(c[:sizeLimit], ' ')

	return string(c[:i]) + "... [truncated]"
}
