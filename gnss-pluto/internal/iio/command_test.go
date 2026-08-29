package iio

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
	t.Setenv("GNSS_PLUTO_IIO_HELPER", "success")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), os.Args[0], []string{"-test.run=TestCommandHelperProcess"}, strings.NewReader("iq-data"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "IQ-DATA" {
		t.Fatalf("stdout = %q, want IQ-DATA", stdout.String())
	}
	if stderr.String() != "diagnostic" {
		t.Fatalf("stderr = %q, want diagnostic", stderr.String())
	}
}

func TestRunIncludesProcessDiagnostics(t *testing.T) {
	t.Setenv("GNSS_PLUTO_IIO_HELPER", "failure")
	err := Run(context.Background(), os.Args[0], []string{"-test.run=TestCommandHelperProcess"}, nil, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "Pluto disconnected") {
		t.Fatalf("Run() error = %v, want process diagnostics", err)
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
		t.Fatalf("tail = %q, want 56789", buffer.String())
	}
}

func TestCommandHelperProcess(t *testing.T) {
	mode := os.Getenv("GNSS_PLUTO_IIO_HELPER")
	if mode == "" {
		return
	}
	if mode == "failure" {
		_, _ = fmt.Fprint(os.Stderr, "Pluto disconnected")
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
