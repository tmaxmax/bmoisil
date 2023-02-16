package pbinfo_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tmaxmax/bmoisil/pkg/pbinfo"
)

func newClient(tb testing.TB, serverURL string) *pbinfo.Client {
	tb.Helper()

	return &pbinfo.Client{
		Client: &http.Client{
			Transport: newTestTransport(serverURL, &http.Transport{}),
		},
	}
}

func TestClient_FindProblemByID(t *testing.T) {
	type test struct {
		name      string
		problemID int
		expect    *pbinfo.Problem
		expectErr bool
		timeout   time.Duration
	}

	tests := []test{
		{
			name:      "Timeout",
			timeout:   time.Millisecond * 5,
			expectErr: true,
		},
		{
			name:      "NotFound",
			expectErr: true,
		},
		{
			name:      "SomeMissing",
			problemID: 100,
			expect: &pbinfo.Problem{
				ID:             100,
				Name:           "NrApPrime",
				Publisher:      "Candale Silviu (silviu)",
				Grade:          9,
				Input:          "nrapprime.in",
				Output:         "nrapprime.out",
				MaxTime:        time.Second / 10,
				MaxMemoryBytes: 64e6,
				MaxStackBytes:  8e6,
				Difficulty:     pbinfo.Easy,
			},
		},
		{
			name:      "AllPresent",
			problemID: 3860,
			expect: &pbinfo.Problem{
				ID:             3860,
				Name:           "consecutive1",
				Publisher:      "Pracsiu Dan (dnprx)",
				Grade:          9,
				Input:          "",
				Output:         "",
				MaxTime:        time.Second,
				MaxMemoryBytes: 256e6,
				MaxStackBytes:  8e6,
				Source:         "EJOI 2021, sesiunea de antrenament",
				Authors:        []string{"Dan Pracsiu"},
				Difficulty:     pbinfo.Contest,
			},
		},
	}

	ts := newTestServer(t)
	defer ts.Close()

	c := newClient(t, ts.URL)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := timeoutContext(tt.timeout)
			defer cancel()

			p, err := c.FindProblemByID(ctx, tt.problemID)
			if tt.expectErr {
				require.Error(t, err, "Expected error on retrieval")
			} else {
				require.NoErrorf(t, err, "Failed to find problem by ID %d", tt.problemID)
				require.Equal(t, tt.expect, p)
			}
		})
	}
}

func TestClient_GetProblemTestCases(t *testing.T) {
	type test struct {
		name        string
		problemID   int
		expectCount int
		expectErr   bool
		timeout     time.Duration
	}

	tests := []test{
		{
			name:      "Timeout",
			timeout:   time.Millisecond * 5,
			expectErr: true,
		},
		{
			name:      "NotFound",
			expectErr: true,
		},
		{
			name:        "Full",
			problemID:   1629,
			expectCount: 10,
		},
		{
			name:        "Examples",
			problemID:   100,
			expectCount: 1,
		},
	}

	ts := newTestServer(t)
	defer ts.Close()

	c := newClient(t, ts.URL)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := timeoutContext(tt.timeout)
			defer cancel()

			cases, err := c.GetProblemTestCases(ctx, tt.problemID)
			if tt.expectErr {
				require.Error(t, err, "Expected error on retrieval")
			} else {
				require.NoErrorf(t, err, "Failed to get cases for ID %d", tt.problemID)
				require.Equal(t, tt.expectCount, len(cases))

				t.Logf("%+v", cases)
			}
		})
	}
}
