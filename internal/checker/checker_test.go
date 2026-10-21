package checker

import (
	"testing"

	"github.com/NEwa-05/traefik-syntax-check/internal/rawdata"
)

func TestCheck_HTTP(t *testing.T) {
	testCases := []struct {
		desc      string
		rule      string
		wantError string
	}{
		{
			desc: "valid single host",
			rule: "Host(`a.domain.com`)",
		},
		{
			desc: "valid combined matchers",
			rule: "Host(`a.domain.com`) && (PathPrefix(`/api`) || Path(`/health`))",
		},
		{
			desc: "valid v3 header matcher",
			rule: "Header(`X-Env`, `prod`)",
		},
		{
			desc:      "multiple hosts",
			rule:      "Host(`a.domain.com`, `b.domain.com`)",
			wantError: "error while adding rule Host: unexpected number of parameters; got 2, expected one of [1]",
		},
		{
			desc:      "multiple path prefixes",
			rule:      "PathPrefix(`/a`, `/b`)",
			wantError: "error while adding rule PathPrefix: unexpected number of parameters; got 2, expected one of [1]",
		},
		{
			desc:      "removed Headers matcher",
			rule:      "Headers(`X-Env`, `prod`)",
			wantError: "unsupported function: Headers",
		},
		{
			desc:      "removed HeadersRegexp matcher",
			rule:      "HeadersRegexp(`X-Env`, `prod.*`)",
			wantError: "unsupported function: HeadersRegexp",
		},
		{
			desc:      "multiple methods",
			rule:      "Method(`GET`, `POST`)",
			wantError: "error while adding rule Method: unexpected number of parameters; got 2, expected one of [1]",
		},
		{
			desc:      "empty rule",
			rule:      "",
			wantError: "empty rule",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			data := &rawdata.RawData{Routers: map[string]rawdata.Router{
				"router@file": {Rule: test.rule},
			}}

			results, err := Check(data, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}

			assertResult(t, results[0], test.wantError)
		})
	}
}

func TestCheck_TCP(t *testing.T) {
	testCases := []struct {
		desc      string
		rule      string
		wantError string
	}{
		{
			desc: "catch-all",
			rule: "HostSNI(`*`)",
		},
		{
			desc: "valid v3 matchers",
			rule: "HostSNIRegexp(`^.+\\.domain\\.com$`) && ALPN(`h2`)",
		},
		{
			desc:      "multiple SNI",
			rule:      "HostSNI(`a.domain.com`, `b.domain.com`)",
			wantError: "unexpected number of parameters; got 2, expected 1",
		},
		{
			desc:      "multiple client IPs",
			rule:      "ClientIP(`10.0.0.1`, `10.0.0.2`)",
			wantError: "unexpected number of parameters; got 2, expected 1",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			data := &rawdata.RawData{TCPRouters: map[string]rawdata.Router{
				"router@file": {Rule: test.rule},
			}}

			results, err := Check(data, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}

			assertResult(t, results[0], test.wantError)
		})
	}
}

func TestCheck_sortAndInternal(t *testing.T) {
	data := &rawdata.RawData{
		Routers: map[string]rawdata.Router{
			"zeta@docker":  {Rule: "Host(`z.com`, `y.com`)"},
			"alpha@file":   {Rule: "Host(`a.com`, `b.com`)"},
			"api@internal": {Rule: "PathPrefix(`/api`)"},
			"beta@docker":  {Rule: "Host(`b.com`)"},
		},
		TCPRouters: map[string]rawdata.Router{
			"db@file": {Rule: "HostSNI(`a.com`, `b.com`)"},
		},
	}

	results, err := Check(data, Options{})
	if err != nil {
		t.Fatal(err)
	}

	got := routers(results)
	want := []string{"http alpha@file", "http beta@docker", "http zeta@docker", "tcp db@file"}
	assertEqual(t, got, want)

	got = routers(Invalid(results))
	want = []string{"http alpha@file", "http zeta@docker", "tcp db@file"}
	assertEqual(t, got, want)

	results, err = Check(data, Options{IncludeInternal: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Errorf("got %d results with internal routers, want 5", len(results))
	}
}

func TestCheck_provider(t *testing.T) {
	data := &rawdata.RawData{Routers: map[string]rawdata.Router{
		"my@router@kubernetescrd": {Rule: "Host(`a.com`)"},
		"noprovider":              {Rule: "Host(`a.com`)"},
	}}

	results, err := Check(data, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Provider != "kubernetescrd" {
		t.Errorf("got provider %q, want %q", results[0].Provider, "kubernetescrd")
	}
	if results[1].Provider != "" {
		t.Errorf("got provider %q, want none", results[1].Provider)
	}
}

func assertResult(t *testing.T, res Result, wantError string) {
	t.Helper()

	if wantError == "" {
		if !res.Valid() {
			t.Errorf("rule %q: unexpected error %q", res.Rule, res.Error)
		}
		return
	}

	if res.Valid() {
		t.Fatalf("rule %q: expected error %q, got none", res.Rule, wantError)
	}
	if res.Error != wantError {
		t.Errorf("rule %q: got error %q, want %q", res.Rule, res.Error, wantError)
	}
}

func routers(results []Result) []string {
	var names []string
	for _, r := range results {
		names = append(names, string(r.Protocol)+" "+r.Router)
	}
	return names
}

func assertEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
