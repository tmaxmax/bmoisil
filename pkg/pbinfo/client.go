package pbinfo

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/gocolly/colly/v2/extensions"
	"golang.org/x/sync/errgroup"
)

const (
	domain       = "pbinfo.ro"
	baseEndpoint = "https://www." + domain
	ajaxEndpoint = baseEndpoint + "/ajx-module"
)

// The Client is used to retrieve data from the PbInfo platform.
// It is comprised of a Colly collector, for scraping HTML pages, and an HTTP client
// for downloading files/fetching endpoints that do not return HTML.
type Client struct {
	// CollectorCacheDir specifies a location where GET requests the scraper makes
	// are cached as files. Caching is disabled if not provided.
	CollectorCacheDir string
	// CollectorDebugger is an optional debugger implementation used by the scraper.
	CollectorDebugger debug.Debugger
	// RoundTripper is, if defined, a custom roundtripper used by the scraper and the HTTP client.
	RoundTripper http.RoundTripper
	// Timeout is the request timeout for both the scraper and the HTTP client.
	// Defaults to no timeout.
	Timeout time.Duration

	collector     *colly.Collector
	collectorInit sync.Once

	client     *http.Client
	clientInit sync.Once
}

func (c *Client) getCollector(ctx context.Context) *colly.Collector {
	c.collectorInit.Do(func() {
		col := colly.NewCollector()
		col.AllowedDomains = []string{domain, "www." + domain}
		col.CacheDir = c.CollectorCacheDir
		col.AllowURLRevisit = true
		col.SetDebugger(c.CollectorDebugger)
		col.WithTransport(c.RoundTripper)
		col.SetRequestTimeout(c.Timeout)
		extensions.RandomUserAgent(col)

		c.collector = col
	})

	col := c.collector.Clone()
	col.Context = ctx
	return col
}

func (c *Client) getClient() *http.Client {
	c.clientInit.Do(func() {
		if c.client == nil {
			c.client = &http.Client{}
		}

		if c.client.Transport == nil {
			c.client.Transport = c.RoundTripper
		}
	})

	return c.client
}

// FindProblemByID returns the information associated with the problem identified by the given ID.
// It returns an error if the problem does not exist, a network error occurs etc.
func (c *Client) FindProblemByID(ctx context.Context, id int) (*Problem, error) {
	col := c.getCollector(ctx)
	p := &Problem{ID: id}

	col.OnHTML(`a[name="section-restrictii"] + table`, func(h *colly.HTMLElement) {
		h.ForEach(`tbody > tr > td`, func(i int, h *colly.HTMLElement) {
			parsers[i](h, p)
		})
	})

	col.OnHTML(`h1.text-primary > a`, func(h *colly.HTMLElement) {
		p.Name = strings.TrimSpace(h.Text)
	})

	if err := col.Visit(fmt.Sprintf("%s/probleme/%d", baseEndpoint, id)); err != nil {
		return nil, fmt.Errorf("failed to find problem with ID %d: %w", id, err)
	}

	return p, nil
}

// GetProblemTestCases returns the test cases for the problem identified by the given ID.
// It firstly tries to retrieve all the test cases, if they are available, otherwise it retrieves
// only the example test cases.
// It returns an error if the problem does not exist, a network error occurs etc.
func (c *Client) GetProblemTestCases(ctx context.Context, problemID int) ([]ProblemTestCase, error) {
	cases, err := c.getProblemFullTestCases(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if len(cases) != 0 {
		return cases, nil
	}

	return c.getProblemExampleTestCases(ctx, problemID)
}

func (c *Client) getProblemFullTestCases(ctx context.Context, problemID int) ([]ProblemTestCase, error) {
	col := c.getCollector(ctx)

	type url struct {
		index int
		href  string
		input bool
	}

	var (
		testCases []ProblemTestCase
		urls      []url
	)

	col.OnHTML(`table > tbody > tr`, func(h *colly.HTMLElement) {
		testCases = append(testCases, ProblemTestCase{})
		index := len(testCases) - 1
		t := &testCases[index]

		h.ForEach(`td`, func(i int, h *colly.HTMLElement) {
			switch i {
			case 1:
				t.Score, _ = strconv.Atoi(normalizeText(h.Text))
			case 2, 3:
				content := h.ChildText(`textarea`)
				if content == "" {
					urls = append(urls, url{index, h.ChildAttr(`a`, `href`), i == 2})
				} else if i == 2 {
					t.Input = []byte(content)
				} else {
					t.Expected = []byte(content)
				}
			case 4:
				if normalizeText(h.Text) == "da" {
					t.IsExample = true
				}
			}
		})
	})

	if err := col.Visit(fmt.Sprintf("%s/ajx-problema-afisare-teste?id=%d", ajaxEndpoint, problemID)); err != nil {
		return nil, fmt.Errorf("failed to retrieve test cases for problem with ID %d: %w", problemID, err)
	}

	if len(testCases) == 0 {
		return nil, nil
	}

	hc := c.getClient()
	g, gctx := errgroup.WithContext(ctx)

	for i := range urls {
		i := i

		g.Go(func() error {
			log.Println(urls[i].href)
			req, err := http.NewRequestWithContext(gctx, http.MethodGet, urls[i].href, nil)
			if err != nil {
				return err
			}

			q := req.URL.Query()
			caseType := q.Get("tip")
			caseID := q.Get("id")

			res, err := hc.Do(req)
			if err != nil {
				return fmt.Errorf("request failed for %s (%s): %w", caseID, caseType, err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				return fmt.Errorf("request failed for %s (%s): %s", caseID, caseType, http.StatusText(res.StatusCode))
			}

			data, err := io.ReadAll(res.Body)
			if err != nil {
				return fmt.Errorf("request failed for %s (%s): %w", caseID, caseType, err)
			}

			if urls[i].input {
				testCases[urls[i].index].Input = data
			} else {
				testCases[urls[i].index].Expected = data
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to retrieve test cases for problem with ID %d: %w", problemID, err)
	}

	for _, c := range testCases {
		if c.Input == nil || c.Expected == nil {
			return nil, nil
		}
	}

	return testCases, nil
}

func (c *Client) getProblemExampleTestCases(ctx context.Context, problemID int) ([]ProblemTestCase, error) {
	col := c.getCollector(ctx)

	var examplesContent []string

	col.OnHTML(`p + pre`, func(h *colly.HTMLElement) {
		examplesContent = append(examplesContent, h.Text)
	})

	if err := col.Visit(fmt.Sprintf("%s/probleme/%d", baseEndpoint, problemID)); err != nil {
		return nil, fmt.Errorf("failed to retrieve examples for problem with ID %d: %w", problemID, err)
	}

	if len(examplesContent)%2 != 0 {
		return nil, fmt.Errorf("failed to retrieve examples for problem with ID %d: incomplete data (%d content chunks instead of even number)", problemID, len(examplesContent))
	}

	testCases := make([]ProblemTestCase, len(examplesContent)/2)
	for i := range testCases {
		t := &testCases[i]
		t.Input = []byte(examplesContent[2*i])
		t.Expected = []byte(examplesContent[2*i+1])
		t.IsExample = true
	}

	return testCases, nil
}

func parseMemoryLimit(input string, output *int64) {
	mem, err := units.FromHumanSize(input)
	if err == nil {
		*output = mem
	}
}

var parsers = [...]func(*colly.HTMLElement, *Problem){
	// publisher
	0: func(h *colly.HTMLElement, p *Problem) {
		p.Publisher = h.ChildText(`span`)
	},
	// grade
	1: func(h *colly.HTMLElement, p *Problem) {
		p.Grade, _ = strconv.Atoi(normalizeText(h.Text))
	},
	// input / output
	2: func(h *colly.HTMLElement, p *Problem) {
		inout := strings.Split(h.ChildText(`span`), "/")
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
	3: func(h *colly.HTMLElement, p *Problem) {
		split := strings.Split(normalizeText(h.Text), " ")
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
	4: func(h *colly.HTMLElement, p *Problem) {
		parseMemoryLimit(h.ChildText(`span[title="Memorie totală"]`), &p.MaxMemoryBytes)
		parseMemoryLimit(h.ChildText(`span[title="Dimensiunea stivei"]`), &p.MaxStackBytes)
	},
	// problem source
	5: func(h *colly.HTMLElement, p *Problem) {
		if text := normalizeText(h.Text); text != "" {
			p.Source = text
		}
	},
	// authors
	6: func(h *colly.HTMLElement, p *Problem) {
		text := normalizeText(h.Text)
		if text == "" {
			return
		}

		p.Authors = strings.Split(text, ",")
		for i, a := range p.Authors {
			p.Authors[i] = strings.TrimSpace(a)
		}
	},
	// difficulty
	7: func(h *colly.HTMLElement, p *Problem) {
		p.Difficulty = ParseProblemDifficulty(normalizeText(h.Text))
	},
	// score
	8: func(h *colly.HTMLElement, p *Problem) {
		text := normalizeText(h.Text)
		if text == "" {
			return
		}

		score, _ := strconv.Atoi(text)
		p.Score = &score
	},
}

func normalizeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "-" {
		return ""
	}
	return text
}
