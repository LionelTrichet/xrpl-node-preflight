package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
)

func sampleReport() model.Report {
	return model.NewReport("validator", "production", []model.Check{
		{ID: "host.os", Status: model.StatusPass, Summary: "Linux", Observed: "linux", Expected: "linux"},
		{ID: "host.memory", Status: model.StatusPass, Summary: "64.0 GiB total memory", Observed: "64.0 GiB", Expected: ">= 64 GiB"},
		{ID: "storage.media", Status: model.StatusWarn, Summary: "storage media could not be determined"},
		{ID: "config.file", Status: model.StatusSkip, Summary: "no configuration file supplied"},
	})
}

func TestTextLayoutIsFixed(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteText(&buffer, sampleReport()); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	output := buffer.String()

	want := []string{
		"XRPL Node Preflight\n",
		"Role:  validator\n",
		"Level: production\n",
		"PASS  host.os                     Linux\n",
		"WARN  storage.media               storage media could not be determined\n",
		"SKIP  config.file                 no configuration file supplied\n",
		"Result: READY WITH WARNINGS\n",
		"4 checks: 2 pass, 1 warn, 0 fail, 1 skip\n",
	}
	for _, fragment := range want {
		if !strings.Contains(output, fragment) {
			t.Errorf("output is missing %q\ngot:\n%s", fragment, output)
		}
	}
}

func TestTextResultNames(t *testing.T) {
	tests := []struct {
		checks []model.Check
		want   string
	}{
		{[]model.Check{{ID: "a", Status: model.StatusPass}}, "Result: READY\n"},
		{[]model.Check{{ID: "a", Status: model.StatusWarn}}, "Result: READY WITH WARNINGS\n"},
		{[]model.Check{{ID: "a", Status: model.StatusFail}}, "Result: NOT READY\n"},
		{[]model.Check{{ID: "a", Status: model.StatusSkip}}, "Result: READY\n"},
	}

	for _, test := range tests {
		var buffer bytes.Buffer
		if err := WriteText(&buffer, model.NewReport("node", "production", test.checks)); err != nil {
			t.Fatalf("WriteText() error = %v", err)
		}
		if !strings.Contains(buffer.String(), test.want) {
			t.Errorf("output = %q, want %q", buffer.String(), test.want)
		}
	}
}

func TestTextHasNoDecoration(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteText(&buffer, sampleReport()); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	output := buffer.String()

	if strings.ContainsRune(output, 0x1b) {
		t.Error("output contains an escape sequence")
	}
	for _, r := range output {
		if r > 0x7e {
			t.Errorf("output contains a non-ASCII rune %q", r)
		}
	}
}

func TestJSONShape(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, sampleReport()); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v", decoded["schemaVersion"])
	}
	if decoded["result"] != model.ResultReadyWithWarnings {
		t.Errorf("result = %v", decoded["result"])
	}

	checks, ok := decoded["checks"].([]any)
	if !ok || len(checks) != 4 {
		t.Fatalf("checks = %v", decoded["checks"])
	}
	first, _ := checks[0].(map[string]any)
	if first["id"] != "host.os" || first["status"] != "pass" {
		t.Errorf("first check = %v", first)
	}

	// An empty observed or expected value is omitted rather than rendered.
	third, _ := checks[2].(map[string]any)
	if _, present := third["observed"]; present {
		t.Errorf("empty observed should be omitted: %v", third)
	}

	summary, _ := decoded["summary"].(map[string]any)
	if summary["pass"] != float64(2) || summary["warn"] != float64(1) || summary["skip"] != float64(1) {
		t.Errorf("summary = %v", summary)
	}

	for _, forbidden := range []string{"timestamp", "duration", "hostname", "runId", "uuid"} {
		if _, present := decoded[forbidden]; present {
			t.Errorf("report carries %q", forbidden)
		}
	}
}

func TestJSONIsDeterministic(t *testing.T) {
	var first, second bytes.Buffer
	if err := WriteJSON(&first, sampleReport()); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := WriteJSON(&second, sampleReport()); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if first.String() != second.String() {
		t.Error("two renderings of the same report differ")
	}
}

func TestWriteErrorsAreReported(t *testing.T) {
	if err := WriteText(failingWriter{}, sampleReport()); err == nil {
		t.Error("WriteText() error = nil, want the write failure")
	}
	if err := WriteJSON(failingWriter{}, sampleReport()); err == nil {
		t.Error("WriteJSON() error = nil, want the write failure")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

var errWrite = errorString("write failed")

type errorString string

func (e errorString) Error() string { return string(e) }
