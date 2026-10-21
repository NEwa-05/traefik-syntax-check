package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NEwa-05/traefik-syntax-check/internal/report"
)

const fixture = "../../testdata/rawdata.json"

func TestRun_text(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-file", fixture}, &stdout, &stderr)
	if code != exitInvalidRules {
		t.Fatalf("got exit code %d, want %d (stderr: %s)", code, exitInvalidRules, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Checked 7 router rule(s)") || !strings.Contains(out, "4 need(s) to be updated") {
		t.Errorf("unexpected summary:\n%s", out)
	}

	// Invalid routers are listed sorted by protocol then name.
	var order []int
	for _, name := range []string{"headers@file", "methods@kubernetescrd", "multi-host@docker", "postgres@file"} {
		order = append(order, strings.Index(out, name))
	}
	for i, idx := range order {
		if idx < 0 || (i > 0 && idx < order[i-1]) {
			t.Fatalf("routers are missing or not sorted:\n%s", out)
		}
	}

	for _, name := range []string{"whoami@docker", "already-v3@file", "catchall@file", "api@internal"} {
		if strings.Contains(out, name) {
			t.Errorf("valid router %s should not be listed:\n%s", name, out)
		}
	}
}

func TestRun_json(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-file", fixture, "-output", "json", "-include-internal"}, &stdout, &stderr)
	if code != exitInvalidRules {
		t.Fatalf("got exit code %d, want %d (stderr: %s)", code, exitInvalidRules, stderr.String())
	}

	var rep report.Report
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Checked != 8 || len(rep.Invalid) != 4 {
		t.Errorf("got %d checked and %d invalid, want 8 and 4", rep.Checked, len(rep.Invalid))
	}
	if rep.Invalid[0].Router != "headers@file" || rep.Invalid[0].Error != "unsupported function: Headers" {
		t.Errorf("unexpected first result: %+v", rep.Invalid[0])
	}
}

func TestRun_allValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rawdata.json")
	if err := os.WriteFile(path, []byte(`{"routers":{"a@file":{"rule":"Host(`+"`a.com`"+`)"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-file", path, "-output", "json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("got exit code %d, want %d (stderr: %s)", code, exitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"invalid": []`) {
		t.Errorf("expected an empty invalid list:\n%s", stdout.String())
	}
}

func TestRun_url(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.ServeFile(rw, req, fixture)
	}))
	t.Cleanup(srv.Close)

	saved := filepath.Join(t.TempDir(), "raw.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-url", srv.URL, "-save-raw", saved}, &stdout, &stderr)
	if code != exitInvalidRules {
		t.Fatalf("got exit code %d, want %d (stderr: %s)", code, exitInvalidRules, stderr.String())
	}

	want, _ := os.ReadFile(fixture)
	got, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("saved raw data differs from the API response")
	}
}

func TestRun_invalidFlags(t *testing.T) {
	testCases := [][]string{
		{},
		{"-url", "http://a", "-file", "b"},
		{"-file", fixture, "-output", "yaml"},
		{"-file", fixture, "-save-raw", "out.json"},
		{"-file", fixture, "extra"},
		{"-header", "invalid"},
		{"-file", "missing.json"},
	}

	for _, args := range testCases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != exitError {
				t.Errorf("got exit code %d, want %d", code, exitError)
			}
		})
	}
}
