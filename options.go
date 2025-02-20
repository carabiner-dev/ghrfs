// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ghrfs

type optFunc func(*Options)

// Options is the configuration struct for the github FS
type Options struct {
	Host         string
	Organization string
	Repository   string
	Cache        bool
	CachePath    string
	Tag          string
}

// Default options
var defaultOptions = Options{
	Host:  githubAPIURL,
	Cache: false,
}

func WithHost(hostname string) optFunc {
	return func(opts *Options) {
		opts.Host = hostname
	}
}

func WithOrganization(org string) optFunc {
	return func(opts *Options) {
		opts.Organization = org
	}
}

func WithRepository(repo string) optFunc {
	return func(opts *Options) {
		opts.Repository = repo
	}
}

func WithTag(tag string) optFunc {
	return func(opts *Options) {
		opts.Tag = tag
	}
}

func WithCache(useCache bool) optFunc {
	return func(opts *Options) {
		opts.Cache = useCache
	}
}

func WithCachePath(path string) optFunc {
	return func(opts *Options) {
		opts.CachePath = path
	}
}
