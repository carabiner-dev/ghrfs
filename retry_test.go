// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/carabiner-dev/github"
	"github.com/stretchr/testify/require"
)

// step is one scripted result returned by scriptedCaller, mimicking the GitHub
// client's caller: an HTTP error carries a non-nil response and a non-nil error,
// while a network error carries a nil response.
type step struct {
	resp *http.Response
	err  error
}

func okStep() step {
	return step{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("body"))}}
}

func httpErrStep(status int) step {
	return step{
		resp: &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))},
		err:  fmt.Errorf("HTTP Error %d sending request", status),
	}
}

func netErrStep() step {
	return step{err: errors.New("dial tcp: connection refused")}
}

type scriptedCaller struct {
	mu    sync.Mutex
	steps []step
	calls int
}

func (s *scriptedCaller) RequestWithContext(_ context.Context, _, _ string, _ io.Reader) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls >= len(s.steps) {
		return nil, errors.New("unexpected extra call")
	}
	st := s.steps[s.calls]
	s.calls++
	return st.resp, st.err
}

func TestGetWithRetry(t *testing.T) {
	for _, tc := range []struct {
		name       string
		retries    uint
		steps      []step
		wantCalls  int
		wantErr    bool
		wantStatus int
	}{
		{"success-first-try", 3, []step{okStep()}, 1, false, http.StatusOK},
		{"retries-5xx-then-succeeds", 3, []step{httpErrStep(http.StatusServiceUnavailable), okStep()}, 2, false, http.StatusOK},
		{"retries-429-then-succeeds", 3, []step{httpErrStep(http.StatusTooManyRequests), okStep()}, 2, false, http.StatusOK},
		{"retries-network-error", 3, []step{netErrStep(), okStep()}, 2, false, http.StatusOK},
		{"no-retry-on-404", 3, []step{httpErrStep(http.StatusNotFound)}, 1, true, 0},
		{"exhausts-retries", 2, []step{httpErrStep(500), httpErrStep(500), httpErrStep(500)}, 3, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caller := &scriptedCaller{steps: tc.steps}
			client, err := github.NewClient(github.WithCaller(caller))
			require.NoError(t, err)

			rfs := &ReleaseFileSystem{
				Options:   Options{Retries: tc.retries},
				retryWait: time.Millisecond, // keep the test fast
			}
			resp, err := rfs.getWithRetry(context.Background(), client, "https://example.com/x")

			require.Equal(t, tc.wantCalls, caller.calls)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	}
}
