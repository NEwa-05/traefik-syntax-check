// Package checker validates router rules against the Traefik v3 rule syntax,
// using the Traefik v3 muxers themselves.
package checker

import (
	"cmp"
	"errors"
	"net/http"
	"slices"
	"strings"

	httpmuxer "github.com/traefik/traefik/v3/pkg/muxer/http"
	tcpmuxer "github.com/traefik/traefik/v3/pkg/muxer/tcp"

	"github.com/NEwa-05/traefik-syntax-check/internal/rawdata"
)

const syntaxV3 = "v3"

// Protocol of a router.
type Protocol string

// Supported protocols.
const (
	HTTP Protocol = "http"
	TCP  Protocol = "tcp"
)

// Result is the validation result of a router rule.
type Result struct {
	Protocol Protocol `json:"protocol"`
	Router   string   `json:"router"`
	Provider string   `json:"provider,omitempty"`
	Rule     string   `json:"rule"`
	// RuleSyntax is the syntax explicitly set on the v2 router, if any.
	RuleSyntax string `json:"ruleSyntax,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Valid reports whether the rule is valid with the v3 syntax.
func (r Result) Valid() bool {
	return r.Error == ""
}

// Options configures the validation.
type Options struct {
	// IncludeInternal also validates the routers from the internal provider.
	IncludeInternal bool
}

// Check validates every HTTP and TCP router rule against the v3 syntax.
// Results are sorted by protocol then router name.
func Check(data *rawdata.RawData, opts Options) ([]Result, error) {
	parser, err := httpmuxer.NewSyntaxParser()
	if err != nil {
		return nil, err
	}

	var results []Result
	for name, router := range data.Routers {
		if skip(name, opts) {
			continue
		}

		// A new muxer is used for each rule as only the parsing outcome matters.
		muxer := httpmuxer.NewMuxer(parser, nil)
		err := muxer.AddRoute(router.Rule, syntaxV3, 0, provider(name), http.NotFoundHandler())
		results = append(results, newResult(HTTP, name, router, err))
	}

	for name, router := range data.TCPRouters {
		if skip(name, opts) {
			continue
		}

		muxer, err := tcpmuxer.NewMuxer(nil)
		if err != nil {
			return nil, err
		}
		err = muxer.AddRoute(router.Rule, syntaxV3, 0, provider(name), nil)
		results = append(results, newResult(TCP, name, router, err))
	}

	Sort(results)

	return results, nil
}

// Invalid returns the results whose rule needs to be updated, preserving order.
func Invalid(results []Result) []Result {
	var invalid []Result
	for _, r := range results {
		if !r.Valid() {
			invalid = append(invalid, r)
		}
	}

	return invalid
}

// Sort sorts results by protocol, then router name.
func Sort(results []Result) {
	slices.SortFunc(results, func(a, b Result) int {
		return cmp.Or(
			cmp.Compare(a.Protocol, b.Protocol),
			cmp.Compare(a.Router, b.Router),
		)
	})
}

func newResult(protocol Protocol, name string, router rawdata.Router, err error) Result {
	res := Result{
		Protocol:   protocol,
		Router:     name,
		Provider:   provider(name),
		Rule:       router.Rule,
		RuleSyntax: router.RuleSyntax,
	}
	switch {
	case strings.TrimSpace(router.Rule) == "":
		res.Error = "empty rule"
	case err != nil:
		res.Error = cause(err, router.Rule).Error()
	}

	return res
}

// cause strips the wrapping errors which only repeat the rule,
// to keep the message focused on what is wrong with it.
func cause(err error, rule string) error {
	if rule == "" {
		return err
	}

	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			return err
		}

		prefix := strings.TrimSuffix(err.Error(), inner.Error())
		if !strings.Contains(prefix, rule) {
			return err
		}
		err = inner
	}
}

func skip(name string, opts Options) bool {
	return !opts.IncludeInternal && provider(name) == "internal"
}

func provider(name string) string {
	if i := strings.LastIndex(name, "@"); i >= 0 {
		return name[i+1:]
	}

	return ""
}
