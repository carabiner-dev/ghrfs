// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromURL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		expect  *Options
		mustErr bool
	}{
		{
			"regular", "https://github.com/protobom/cel/releases/tag/v0.5.0",
			&Options{Host: "api.github.com", Organization: "protobom", Repository: "cel", Tag: "v0.5.0"}, false,
		},
		{"url-bad", "Chill, there is nothing here", nil, true},
		{"url-other", "https://github.com/protobom/protobom/stargazers", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := Options{}
			err := FromURL(tc.input)(&o)
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect.Host, o.Host)
			require.Equal(t, tc.expect.Repository, o.Repository)
			require.Equal(t, tc.expect.Organization, o.Organization)
			require.Equal(t, tc.expect.Organization, o.Organization)
		})
	}
}
