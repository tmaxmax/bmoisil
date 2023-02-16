package pbinfo

import (
	"strings"
	"time"

	"github.com/docker/go-units"
)

// ProblemDifficulty represents the difficulty of a problem.
type ProblemDifficulty int

const (
	Unknown ProblemDifficulty = iota
	Easy
	Medium
	Difficult
	Contest

	difficultyEasyString      = "easy"
	difficultyMediumString    = "medium"
	difficultyDifficultString = "difficult"
	difficultyContestString   = "contest"
	difficultyUnknownString   = "unknown"
)

func (p ProblemDifficulty) String() string {
	switch p {
	case Easy:
		return difficultyEasyString
	case Medium:
		return difficultyMediumString
	case Difficult:
		return difficultyDifficultString
	case Contest:
		return difficultyContestString
	default:
		return difficultyUnknownString
	}
}

var asciiReplacer = strings.NewReplacer("ă", "a", "â", "a", "î", "i", "ș", "s", "ț", "t")

// ParseProblemDifficulty tries to determine the difficulty of a problem from the given string.
// It normalizes the input by converting it to lowercase and converting letters with diacritics
// to their closest ASCII equivalent, and then it compares the result with certain predefined values
// associated with difficulty levels. If no matching value is found, the Unknown difficulty is returned.
func ParseProblemDifficulty(input string) ProblemDifficulty {
	switch asciiReplacer.Replace(strings.ToLower(input)) {
	case difficultyEasyString, "usoara", "usor":
		return Easy
	case difficultyMediumString, "medie", "mediu":
		return Medium
	case difficultyDifficultString, "dificila", "dificil":
		return Difficult
	case difficultyContestString, "concurs":
		return Contest
	default:
		return Unknown
	}
}

// Problem represents a single PbInfo problem.
type Problem struct {
	// The ID of the problem.
	ID int
	// The name of the problem.
	Name string
	// The person that published this problem.
	Publisher string
	// The grade the problem is targeted at. If 0, grade is unknown.
	Grade int
	// The input file. Is empty if the input is from STDIN.
	Input string
	// The output file. It is empty if the output is to STDOUT.
	Output string
	// The maximum execution time of the solution. If 0, no time limit is given.
	MaxTime time.Duration
	// The maximum amount of both stack and heap memory used by the solution, in bytes.
	// If 0, no memory limit is given.
	MaxMemoryBytes int64
	// The maximum amount of stack memory used by the solution, in bytes.
	// If 0, no memory limit is given.
	MaxStackBytes int64
	// The source of the problem: the place where it was taken from. If empty, source is unknown.
	Source string
	// The authors of the problem. If empty/nil, the author is considered the poster.
	Authors []string
	// The difficulty of the problem.
	Difficulty ProblemDifficulty
	// The score of the latest uploaded solution. It is nil if no solution was uploaded before.
	Score *int
}

// ReadableMaxMemory returns the memory limit of the solution in a human-readable format.
func (p *Problem) ReadableMaxMemory() string {
	return units.HumanSize(float64(p.MaxMemoryBytes))
}

// ReadableMaxStack returns the stack memory limit of the solution in a human-readable format.
func (p *Problem) ReadableMaxStack() string {
	return units.HumanSize(float64(p.MaxStackBytes))
}

// InputFromStdin indicates whether the problem input should be read from standard input or not.
// If this is not true, Problem.Input indicates the expected input filename.
func (p *Problem) InputFromStdin() bool {
	return p.Input == ""
}

// OutputToStdout indicates whether the problem output should be written to standard output or not.
// If this is not true, Problem.Output indicates the expected output filename.
func (p *Problem) OutputToStdout() bool {
	return p.Output == ""
}

// TestCase holds the input and the expected output for a single test case of a problem.
type TestCase struct {
	Input     []byte
	Output    []byte
	IsExample bool
	Score     int
}
