// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithToken(t *testing.T) {
	t.Parallel()
	opts := Options{}
	require.NoError(t, WithToken("s3cr3t")(&opts))
	require.Equal(t, "s3cr3t", opts.Token)
}

// TestGetClientForURL ensures the client used to download release assets is
// configured with the token (required for private releases) and points at the
// asset host rather than the API host.
func TestGetClientForURL(t *testing.T) {
	t.Parallel()
	c, err := getClientForURL(
		"https://github.com/example/repo/releases/download/v1.0.0/asset.txt", "s3cr3t",
	)
	require.NoError(t, err)
	require.Equal(t, "s3cr3t", c.Options.Token)
	require.Equal(t, "github.com", c.Options.Host)
}
