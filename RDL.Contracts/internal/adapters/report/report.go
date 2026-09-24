// Package report escribe el resultado de un comando en texto (para personas) o JSON (para el CI).
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

type Format string

const (
	Text Format = "text"
	JSON Format = "json"
)

func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case Text, JSON:
		return Format(s), nil
	}
	return "", fmt.Errorf("formato %q desconocido (text o json)", s)
}

func Write(w io.Writer, f Format, command string, res app.Result) error {
	if f == JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Command  string `json:"command"`
			Passed   bool   `json:"passed"`
			Findings any    `json:"findings"`
		}{command, !res.Failed(), nonNil(res)})
	}
	return writeText(w, command, res)
}

func writeText(w io.Writer, command string, res app.Result) error {
	var errs, warns int
	for _, f := range res.Findings {
		loc := f.File
		if f.Pointer != "" {
			loc += "#" + f.Pointer
		}
		if _, err := fmt.Fprintf(w, "%-7s %s [%s] %s\n", f.Severity, loc, f.Rule, f.Message); err != nil {
			return err
		}
		if f.Severity == finding.Error {
			errs++
		} else {
			warns++
		}
	}
	verdict := "OK"
	if res.Failed() {
		verdict = "FALLA"
	}
	_, err := fmt.Fprintf(w, "%s: %s (%d errores, %d avisos)\n", command, verdict, errs, warns)
	return err
}

func nonNil(res app.Result) any {
	if res.Findings == nil {
		return []struct{}{}
	}
	return res.Findings
}
