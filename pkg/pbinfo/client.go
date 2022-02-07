package pbinfo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/docker/go-units"
	"github.com/tmaxmax/bmoisil/pkg/traverse"
	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"
)

const (
	domain       = "pbinfo.ro"
	baseEndpoint = "https://www." + domain
	ajaxEndpoint = baseEndpoint + "/ajx-module"
)

// The Client is used to retrieve data from the PbInfo platform.
type Client struct {
	// The HTTP client to use. Defaults to http.DefaultClient.
	Client     *http.Client
	clientInit sync.Once
}

func (c *Client) request(req *http.Request) (*http.Response, error) {
	c.clientInit.Do(func() {
		if c.Client == nil {
			c.Client = http.DefaultClient
		}
	})

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request to %q: %w", req.URL, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response from %q: %d %s", req.URL, res.StatusCode, http.StatusText(res.StatusCode))
	}

	return res, nil
}

func (c *Client) requestHTML(ctx context.Context, url string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate request to %q: %w", url, err)
	}

	res, err := c.request(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	root, err := html.Parse(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body for %q: %w", req.URL, err)
	}

	return root, nil
}

var (
	selectorProblemMetadataTable  = cascadia.MustCompile(`a[name="section-restrictii"] + table`)
	selectorProblemMetadataColumn = cascadia.MustCompile(`tbody > tr > td`)
	selectorProblemTitle          = cascadia.MustCompile(`h1.text-primary > a`)
)

// FindProblemByID returns the information associated with the problem identified by the given ID.
// It returns an error if the problem does not exist, a network error occurs etc.
func (c *Client) FindProblemByID(ctx context.Context, id int) (*Problem, error) {
	root, err := c.requestHTML(ctx, fmt.Sprintf("%s/probleme/%d", baseEndpoint, id))
	if err != nil {
		return nil, fmt.Errorf("pbinfo: %w", err)
	}

	table := selectorProblemMetadataTable.MatchFirst(root)
	if table == nil {
		return nil, fmt.Errorf("pbinfo: FindProblemByID failed to find info table for problem %d: HTML changed?", id)
	}

	columns := selectorProblemMetadataColumn.MatchAll(table)
	if l := len(columns); l < 8 || l > 9 {
		return nil, fmt.Errorf("pbinfo: FindProblemByID failed to find info columns for problem %d: HTML changed?", id)
	}

	p := &Problem{ID: id}
	for i, col := range columns {
		parsers[i](col, p)
	}

	title := selectorProblemTitle.MatchFirst(root)
	if title == nil {
		return nil, fmt.Errorf("pbinfo: FindProblemByID failed to find title for problem %d: HTML changed?", id)
	}

	p.Name = text(title)

	return p, nil
}

// GetProblemTestCases returns the test cases for the problem identified by the given ID.
// It firstly tries to retrieve all the test cases, if they are available, otherwise it retrieves
// only the example test cases.
// It returns an error if the problem does not exist, a network error occurs etc.
func (c *Client) GetProblemTestCases(ctx context.Context, problemID int) ([]TestCase, error) {
	cases, err := c.getProblemFullTestCases(ctx, problemID)
	// If an error is returned, getProblemExampleTestCases will probably fail too (e.g. network error).
	if err != nil {
		return nil, fmt.Errorf("pbinfo: %w", err)
	}
	if len(cases) != 0 {
		return cases, nil
	}

	// Try to retrieve examples if no test cases were found.
	cases, err = c.getProblemExampleTestCases(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("pbinfo: %w", err)
	}

	return cases, nil
}

var (
	selectorTestCasesTableRows = cascadia.MustCompile(`table > tbody > tr`)
	selectorTableColumn        = cascadia.MustCompile(`td`)
	selectorTextArea           = cascadia.MustCompile(`textarea`)
	selectorAnchor             = cascadia.MustCompile(`a`)
)

func (c *Client) getProblemFullTestCases(ctx context.Context, problemID int) ([]TestCase, error) {
	root, err := c.requestHTML(ctx, fmt.Sprintf("%s/ajx-problema-afisare-teste.php?id=%d", ajaxEndpoint, problemID))
	if err != nil {
		return nil, err
	}

	type url struct {
		index int
		href  string
		input bool
	}

	var (
		testCases []TestCase
		urls      []url
	)

	for _, row := range selectorTestCasesTableRows.MatchAll(root) {
		testCases = append(testCases, TestCase{})
		index := len(testCases) - 1
		t := &testCases[index]

		for i, col := range selectorTableColumn.MatchAll(row) {
			switch i {
			case 1:
				t.Score, _ = strconv.Atoi(text(col))
			case 2, 3:
				content := childText(col, selectorTextArea)
				if content == "" {
					urls = append(urls, url{index, childAttr(col, selectorAnchor, `href`), i == 2})
				} else if i == 2 {
					t.Input = []byte(content)
				} else {
					t.Output = []byte(content)
				}
			case 4:
				if dashToEmpty(text(col)) == "da" {
					t.IsExample = true
				}
			}
		}
	}

	if len(testCases) == 0 {
		// No test cases found here, return both the slice and the error as nil
		// so GetProblemTestCases knows to call getProblemExampleTestCases.
		return nil, nil
	}

	g, gctx := errgroup.WithContext(ctx)

	for i := range urls {
		i := i

		g.Go(func() error {
			url := fmt.Sprintf("%s%s", baseEndpoint, urls[i].href)
			req, err := http.NewRequestWithContext(gctx, http.MethodGet, url, nil)
			if err != nil {
				return err
			}

			q := req.URL.Query()
			caseType := q.Get("tip")
			caseID := q.Get("id")

			res, err := c.request(req)
			if err != nil {
				return fmt.Errorf("request failed for %s (%s): %w", caseID, caseType, err)
			}
			defer res.Body.Close()

			data, err := io.ReadAll(res.Body)
			if err != nil {
				return fmt.Errorf("request failed for %s (%s): %w", caseID, caseType, err)
			}

			if urls[i].input {
				testCases[urls[i].index].Input = data
			} else {
				testCases[urls[i].index].Output = data
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to retrieve test cases for problem with ID %d: %w", problemID, err)
	}

	for _, c := range testCases {
		if c.Input == nil || c.Output == nil {
			return nil, nil
		}
	}

	return testCases, nil
}

var selectorExamples = cascadia.MustCompile(`p + pre`)

func (c *Client) getProblemExampleTestCases(ctx context.Context, problemID int) ([]TestCase, error) {
	root, err := c.requestHTML(ctx, fmt.Sprintf("%s/probleme/%d", baseEndpoint, problemID))
	if err != nil {
		return nil, err
	}

	var examplesContent []string

	for _, example := range selectorExamples.MatchAll(root) {
		examplesContent = append(examplesContent, text(example))
	}

	if len(examplesContent)%2 != 0 {
		return nil, fmt.Errorf("failed to retrieve examples for problem with ID %d: incomplete data (%d content chunks instead of even number)", problemID, len(examplesContent))
	}

	testCases := make([]TestCase, len(examplesContent)/2)
	for i := range testCases {
		t := &testCases[i]
		t.Input = []byte(examplesContent[2*i])
		t.Output = []byte(examplesContent[2*i+1])
		t.IsExample = true
	}

	return testCases, nil
}

var (
	selectorSpan           = cascadia.MustCompile(`span`)
	selectorTotalMemory    = cascadia.MustCompile(`span[title="Memorie totală"]`)
	selectorStackDimension = cascadia.MustCompile(`span[title="Dimensiunea stivei"]`)
)

func dashToEmpty(text string) string {
	if text == "-" {
		return ""
	}
	return text
}

func text(n *html.Node) string {
	sb := strings.Builder{}

	traverse.Depth(n, func(n *html.Node) bool {
		if n.Type != html.TextNode {
			if text := strings.TrimSpace(n.Data); text != "" {
				_, _ = sb.WriteString(text)
			}
		}

		return true
	})

	return sb.String()
}

func childText(n *html.Node, childSelector cascadia.Selector) string {
	child := childSelector.MatchFirst(n)
	if child == nil {
		return ""
	}

	return text(child)
}

func childAttr(n *html.Node, childSelector cascadia.Selector, attr string) string {
	child := childSelector.MatchFirst(n)
	if child == nil {
		return ""
	}

	for _, a := range child.Attr {
		if a.Key == attr {
			return a.Val
		}
	}

	return ""
}

func parseMemoryLimit(input string, output *int64) {
	mem, err := units.FromHumanSize(input)
	if err == nil {
		*output = mem
	}
}

var parsers = [...]func(*html.Node, *Problem){
	// publisher
	0: func(n *html.Node, p *Problem) {
		p.Publisher = childText(n, selectorSpan)
	},
	// grade
	1: func(n *html.Node, p *Problem) {
		p.Grade, _ = strconv.Atoi(dashToEmpty(text(n)))
	},
	// input / output
	2: func(n *html.Node, p *Problem) {
		inout := strings.Split(childText(n, selectorSpan), "/")
		if len(inout) != 2 {
			p.Input = "-"
			return
		}

		in := strings.TrimSpace(inout[0])
		out := strings.TrimSpace(inout[1])

		if in == "tastatură" || out == "ecran" {
			p.Input = "-"
			return
		}

		p.Input = in
		p.Output = out
	},
	// time limit
	3: func(n *html.Node, p *Problem) {
		split := strings.Split(dashToEmpty(text(n)), " ")
		if len(split) < 1 {
			return
		}

		rawTime, err := strconv.ParseFloat(split[0], 64)
		if err != nil {
			return
		}

		// TODO: check if there are cases where time isn't expressed in seconds
		p.MaxTime = time.Duration(rawTime * float64(time.Second))
	},
	// memory limits
	4: func(n *html.Node, p *Problem) {
		parseMemoryLimit(childText(n, selectorTotalMemory), &p.MaxMemoryBytes)
		parseMemoryLimit(childText(n, selectorStackDimension), &p.MaxStackBytes)
	},
	// problem source
	5: func(n *html.Node, p *Problem) {
		if t := dashToEmpty(text(n)); t != "" {
			p.Source = t
		}
	},
	// authors
	6: func(n *html.Node, p *Problem) {
		t := dashToEmpty(text(n))
		if t == "" {
			return
		}

		p.Authors = strings.Split(t, ",")
		for i, a := range p.Authors {
			p.Authors[i] = strings.TrimSpace(a)
		}
	},
	// difficulty
	7: func(n *html.Node, p *Problem) {
		p.Difficulty = ParseProblemDifficulty(dashToEmpty(text(n)))
	},
	// score
	8: func(n *html.Node, p *Problem) {
		t := dashToEmpty(text(n))
		if t == "" {
			return
		}

		score, _ := strconv.Atoi(t)
		p.Score = &score
	},
}
