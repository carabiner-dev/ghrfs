// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

import (
	"fmt"
	"net/url"
	"regexp"
)

type optFunc func(*Options) error

// Options is the configuration struct for the github FS
type Options struct {
	Cache             bool
	ParallelDownloads int
	Host              string
	Organization      string
	Repository        string
	CachePath         string
	Tag               string
}

// Default options
var defaultOptions = Options{
	Host:              githubAPIURL,
	Cache:             false,
	ParallelDownloads: 3,
}

const releasePathPattern = `/([A-Za-z0-9-_\.]+)/([A-Za-z0-9-_\.]+)/releases/tag/(\S+)`

var releasePathRegex *regexp.Regexp

// FromURL intializaes thew options set from a github release URL
func FromURL(urlString string) optFunc {
	return func(o *Options) error {
		u, err := url.Parse(urlString)
		if err != nil {
			return err
		}

		if releasePathRegex == nil {
			releasePathRegex = regexp.MustCompile(releasePathPattern)
		}

		pts := releasePathRegex.FindStringSubmatch(u.Path)
		if pts == nil {
			return fmt.Errorf("URL does not point to a release")
		}

		// Sset the bits from the URL
		o.Organization = pts[1]
		o.Repository = pts[2]
		o.Tag = pts[3]

		// If the host is github, then we set the github endpoint hostname
		// for the API client.
		if u.Hostname() == "github.com" {
			o.Host = githubAPIURL
		}
		return nil
	}
}

func WithHost(hostname string) optFunc {
	return func(opts *Options) error {
		opts.Host = hostname
		return nil
	}
}

func WithOrganization(org string) optFunc {
	return func(opts *Options) error {
		opts.Organization = org
		return nil
	}
}

func WithRepository(repo string) optFunc {
	return func(opts *Options) error {
		opts.Repository = repo
		return nil
	}
}

func WithTag(tag string) optFunc {
	return func(opts *Options) error {
		opts.Tag = tag
		return nil
	}
}

func WithCache(useCache bool) optFunc {
	return func(opts *Options) error {
		opts.Cache = useCache
		return nil
	}
}

func WithCachePath(path string) optFunc {
	return func(opts *Options) error {
		opts.CachePath = path
		return nil
	}
}

func WithParallelDownloads(dl int) optFunc {
	return func(opts *Options) error {
		opts.ParallelDownloads = dl
		return nil
	}
}
