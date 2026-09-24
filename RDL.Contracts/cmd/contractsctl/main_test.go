package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		code int
		out  string
	}{
		{nil, exitUsage, "uso:"},
		{[]string{"version"}, exitOK, "dev"},
		{[]string{"desconocido"}, exitUsage, "comando desconocido"},
		{[]string{"validate", "-root", "../.."}, exitOK, "validate: OK"},
		{[]string{"validate", "-root", "testdata/vacio"}, exitFindings, "ownership-missing"},
		{[]string{"lint", "-root", "../..", "-format", "json"}, exitOK, `"passed": true`},
		{[]string{"lint", "-root", "testdata/vacio"}, exitFindings, "problem-registry"},
		{[]string{"breaking", "-base", "../..", "-root", "../.."}, exitOK, "breaking: OK"},
		{[]string{"breaking", "-root", "../.."}, exitUsage, "uso: contractsctl breaking"},
		{[]string{"breaking", "-base", "testdata/vacio", "-root", "../.."}, exitUsage, "no es un repo de contratos"},
		{[]string{"diagram", "-root", "../..", "state-machines/payment.yaml"}, exitOK, "stateDiagram-v2"},
		{[]string{"validate", "-format", "xml"}, exitUsage, "formato"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		code := run(context.Background(), c.args, &out, &errOut)
		if code != c.code {
			t.Errorf("%v: código %d, se esperaba %d (stderr: %s)", c.args, code, c.code, errOut.String())
		}
		if !strings.Contains(out.String()+errOut.String(), c.out) {
			t.Errorf("%v: falta %q en la salida:\n%s%s", c.args, c.out, out.String(), errOut.String())
		}
	}
}
