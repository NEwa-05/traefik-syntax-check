// Command tsc (traefik-syntax-check) gathers the routers of a running Traefik v2
// instance and reports the rules which are not valid with the Traefik v3 syntax.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/NEwa-05/traefik-syntax-check/internal/checker"
	"github.com/NEwa-05/traefik-syntax-check/internal/rawdata"
	"github.com/NEwa-05/traefik-syntax-check/internal/report"
)

// Exit codes.
const (
	exitOK           = 0
	exitInvalidRules = 1
	exitError        = 2
)

const traefikModule = "github.com/traefik/traefik/v3"

type headerFlag http.Header

func (h headerFlag) String() string { return "" }

func (h headerFlag) Set(value string) error {
	name, val, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid header %q, expected \"Name: value\"", value)
	}
	http.Header(h).Add(strings.TrimSpace(name), strings.TrimSpace(val))

	return nil
}

type config struct {
	url             string
	file            string
	user            string
	headers         http.Header
	insecure        bool
	timeout         time.Duration
	output          string
	saveRaw         string
	includeInternal bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg := config{headers: http.Header{}}

	fs := flag.NewFlagSet("tsc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, `tsc (traefik-syntax-check) reports the Traefik v2 router rules to update for Traefik v3.

Usage:
  tsc -url https://traefik-dashboard.domain.com [flags]
  tsc -file rawdata.json [flags]

Exit codes: %d all rules are valid, %d some rules need to be updated, %d error.

Flags:
`, exitOK, exitInvalidRules, exitError)
		fs.PrintDefaults()
	}

	var version bool
	fs.StringVar(&cfg.url, "url", "", "Traefik v2 API or dashboard base URL, /api/rawdata is appended when missing")
	fs.StringVar(&cfg.file, "file", "", "read the raw data from a JSON file saved from /api/rawdata instead of -url")
	fs.StringVar(&cfg.user, "user", "", "basic auth credentials as user:password")
	fs.Var(headerFlag(cfg.headers), "header", "extra HTTP header as \"Name: value\" (repeatable)")
	fs.BoolVar(&cfg.insecure, "insecure", false, "skip TLS certificate verification")
	fs.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	fs.StringVar(&cfg.output, "output", "text", "output format: text or json")
	fs.StringVar(&cfg.saveRaw, "save-raw", "", "save the raw data fetched from -url to this file")
	fs.BoolVar(&cfg.includeInternal, "include-internal", false, "also check the routers from the internal provider")
	fs.BoolVar(&version, "version", false, "print the Traefik v3 version used for the validation and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	if version {
		fmt.Fprintf(stdout, "Traefik %s rule syntax\n", traefikVersion())
		return exitOK
	}

	if err := validate(cfg, fs.NArg()); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		fs.Usage()
		return exitError
	}

	code, err := check(cfg, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitError
	}

	return code
}

func validate(cfg config, nArgs int) error {
	switch {
	case nArgs > 0:
		return errors.New("unexpected positional arguments")
	case cfg.url == "" && cfg.file == "":
		return errors.New("one of -url or -file is required")
	case cfg.url != "" && cfg.file != "":
		return errors.New("-url and -file are mutually exclusive")
	case cfg.saveRaw != "" && cfg.url == "":
		return errors.New("-save-raw requires -url")
	case cfg.output != "text" && cfg.output != "json":
		return fmt.Errorf("unsupported output format %q", cfg.output)
	}

	return nil
}

func check(cfg config, stdout io.Writer) (int, error) {
	data, err := load(cfg)
	if err != nil {
		return exitError, err
	}

	results, err := checker.Check(data, checker.Options{IncludeInternal: cfg.includeInternal})
	if err != nil {
		return exitError, fmt.Errorf("creating the v3 rule parsers: %w", err)
	}

	rep := report.New(traefikVersion(), results)
	if cfg.output == "json" {
		err = report.WriteJSON(stdout, rep)
	} else {
		err = report.WriteText(stdout, rep)
	}
	if err != nil {
		return exitError, fmt.Errorf("writing report: %w", err)
	}

	if len(rep.Invalid) > 0 {
		return exitInvalidRules, nil
	}

	return exitOK, nil
}

func load(cfg config) (*rawdata.RawData, error) {
	if cfg.file != "" {
		return rawdata.ReadFile(cfg.file)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	data, body, err := rawdata.Fetch(ctx, cfg.url, rawdata.FetchOptions{
		Headers:   cfg.headers,
		BasicAuth: cfg.user,
		Insecure:  cfg.insecure,
		Timeout:   cfg.timeout,
	})
	if err != nil {
		return nil, err
	}

	if cfg.saveRaw != "" {
		if err := os.WriteFile(cfg.saveRaw, body, 0o600); err != nil {
			return nil, fmt.Errorf("saving raw data: %w", err)
		}
	}

	return data, nil
}

// traefikVersion returns the version of the Traefik v3 module the binary is built with.
func traefikVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == traefikModule {
				return dep.Version
			}
		}
	}

	return "v3"
}
