package rawdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestURL(t *testing.T) {
	testCases := []struct {
		base    string
		want    string
		wantErr bool
	}{
		{base: "https://traefik.domain.com", want: "https://traefik.domain.com/api/rawdata"},
		{base: "https://traefik.domain.com/", want: "https://traefik.domain.com/api/rawdata"},
		{base: "https://traefik.domain.com/dashboard/", want: "https://traefik.domain.com/api/rawdata"},
		{base: "https://traefik.domain.com/api", want: "https://traefik.domain.com/api/rawdata"},
		{base: "https://traefik.domain.com/api/rawdata", want: "https://traefik.domain.com/api/rawdata"},
		{base: "http://localhost:8080/traefik/", want: "http://localhost:8080/traefik/api/rawdata"},
		{base: "traefik.domain.com", wantErr: true},
		{base: "://bad", wantErr: true},
	}

	for _, test := range testCases {
		t.Run(test.base, func(t *testing.T) {
			got, err := URL(test.base)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/rawdata" {
			http.NotFound(rw, req)
			return
		}
		if user, pass, ok := req.BasicAuth(); !ok || user != "admin" || pass != "secret" {
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}
		if req.Header.Get("X-Custom") != "value" {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		http.ServeFile(rw, req, "../../testdata/rawdata.json")
	}))
	t.Cleanup(srv.Close)

	opts := FetchOptions{
		Headers:   http.Header{"X-Custom": []string{"value"}},
		BasicAuth: "admin:secret",
		Timeout:   5 * time.Second,
	}
	data, body, err := Fetch(context.Background(), srv.URL+"/dashboard/", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Error("expected the raw body to be returned")
	}
	if len(data.Routers) != 6 || len(data.TCPRouters) != 2 {
		t.Errorf("got %d HTTP and %d TCP routers, want 6 and 2", len(data.Routers), len(data.TCPRouters))
	}

	router := data.Routers["already-v3@file"]
	if router.Rule != "Header(`X-Env`, `prod`)" || router.RuleSyntax != "v3" || router.Status != "enabled" {
		t.Errorf("unexpected router: %+v", router)
	}

	opts.BasicAuth = "admin:wrong"
	if _, _, err := Fetch(context.Background(), srv.URL, opts); err == nil {
		t.Error("expected an error on unauthorized response")
	}
}

func TestReadFile(t *testing.T) {
	data, err := ReadFile("../../testdata/rawdata.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Routers) != 6 || len(data.TCPRouters) != 2 {
		t.Errorf("got %d HTTP and %d TCP routers, want 6 and 2", len(data.Routers), len(data.TCPRouters))
	}

	if _, err := ReadFile("../../testdata/missing.json"); err == nil {
		t.Error("expected an error on missing file")
	}
}
