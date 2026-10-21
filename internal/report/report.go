// Package report renders the validation results.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/NEwa-05/traefik-syntax-check/internal/checker"
)

// Report is the outcome of a validation run.
type Report struct {
	// TraefikVersion is the Traefik v3 version providing the rule syntax.
	TraefikVersion string `json:"traefikVersion"`
	Checked        int    `json:"checked"`
	// Invalid holds the routers whose rule needs to be updated, sorted.
	Invalid []checker.Result `json:"invalid"`
}

// New builds a report from the sorted validation results.
func New(traefikVersion string, results []checker.Result) Report {
	invalid := checker.Invalid(results)
	if invalid == nil {
		invalid = []checker.Result{}
	}

	return Report{
		TraefikVersion: traefikVersion,
		Checked:        len(results),
		Invalid:        invalid,
	}
}

// WriteText writes a human-readable report.
func WriteText(w io.Writer, r Report) error {
	if _, err := fmt.Fprintf(w, "Checked %d router rule(s) against Traefik %s syntax: %d need(s) to be updated.\n",
		r.Checked, r.TraefikVersion, len(r.Invalid)); err != nil {
		return err
	}
	if len(r.Invalid) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "PROTOCOL\tROUTER\tRULE\tERROR")
	for _, res := range r.Invalid {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", res.Protocol, res.Router, res.Rule, res.Error)
	}

	return tw.Flush()
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	return enc.Encode(r)
}
