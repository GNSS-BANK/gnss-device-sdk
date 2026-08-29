package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunStreamsStandardInputAndOutput(t *testing.T) {
	t.Setenv("GNSS_HACKRF_HELPER", "success")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(
		context.Background(),
		os.Args[0],
		[]string{"-test.run=TestCommandHelperProcess"},
		strings.NewReader("iq-data"),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "IQ-DATA" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "IQ-DATA")
	}
	if stderr.String() != "diagnostic" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "diagnostic")
	}
}

func TestRunIncludesProcessDiagnostics(t *testing.T) {
	t.Setenv("GNSS_HACKRF_HELPER", "failure")

	err := Run(
		context.Background(),
		os.Args[0],
		[]string{"-test.run=TestCommandHelperProcess"},
		nil,
		io.Discard,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "device failed") {
		t.Fatalf("Run() error = %v, want device diagnostics", err)
	}
}

func TestRunReturnsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, os.Args[0], []string{"-test.run=TestCommandHelperProcess"}, nil, io.Discard, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestTailBufferKeepsBoundedSuffix(t *testing.T) {
	buffer := &tailBuffer{limit: 5}
	for _, value := range []string{"12", "3456", "789"} {
		if _, err := buffer.Write([]byte(value)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if buffer.String() != "56789" {
		t.Fatalf("tail = %q, want %q", buffer.String(), "56789")
	}
}

func TestCommandHelperProcess(t *testing.T) {
	mode := os.Getenv("GNSS_HACKRF_HELPER")
	if mode == "" {
		return
	}

	if mode == "failure" {
		_, _ = fmt.Fprint(os.Stderr, "device failed")
		os.Exit(7)
	}

	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(8)
	}
	_, _ = os.Stdout.Write(bytes.ToUpper(payload))
	_, _ = fmt.Fprint(os.Stderr, "diagnostic")
	os.Exit(0)
}
