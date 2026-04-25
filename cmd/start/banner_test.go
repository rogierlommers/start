package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintBannerIncludesBuildTime(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	oldBuildTime := appBuildTime
	appBuildTime = "test-build-time"
	t.Cleanup(func() {
		appBuildTime = oldBuildTime
	})

	printBanner()
	if err := w.Close(); err != nil {
		t.Fatalf("stdout writer close error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-build-time") {
		t.Fatalf("banner output missing build time: %q", output)
	}
	if !strings.Contains(output, "███████") {
		t.Fatalf("banner output missing banner art: %q", output)
	}
}
