package closeutil

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestLogCloseCallsCloser(t *testing.T) {
	called := false

	LogClose("thing", func() error {
		called = true
		return nil
	})

	if !called {
		t.Fatal("expected the closer to be called")
	}
}

func TestLogCloseLogsError(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)

	LogClose("thing", func() error { return errors.New("boom") })

	if got := buf.String(); !strings.Contains(got, "close thing") || !strings.Contains(got, "boom") {
		t.Fatalf("expected log output to mention the close failure, got %q", got)
	}
}
