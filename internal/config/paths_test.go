package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUserHome(t *testing.T) {
	home := userHome()
	if home == "" {
		t.Fatal("userHome() returned empty string")
	}
	// Must be an absolute path.
	if !filepath.IsAbs(home) {
		t.Fatalf("userHome() not absolute: %q", home)
	}
}

func TestDefaultConfigDir(t *testing.T) {
	dir := defaultConfigDir()
	if dir == "" {
		t.Fatal("defaultConfigDir() empty")
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("defaultConfigDir() not absolute: %q", dir)
	}
	if runtime.GOOS == "windows" {
		// Expect somewhere under AppData\Roaming\agent or similar.
		lower := strings.ToLower(dir)
		if !strings.Contains(lower, "agent") {
			t.Fatalf("windows config dir should contain 'agent': %q", dir)
		}
	} else {
		if !strings.HasSuffix(dir, filepath.Join(".config", "agent")) &&
			!strings.Contains(dir, "agent") {
			t.Fatalf("unix config dir unexpected: %q", dir)
		}
	}
}

func TestDefaultSessionPath(t *testing.T) {
	p := defaultSessionPath()
	if p == "" {
		t.Fatal("defaultSessionPath() empty")
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("defaultSessionPath() not absolute: %q", p)
	}
}

func TestExpandHomePath(t *testing.T) {
	home := userHome()
	if home == "" {
		t.Skip("no home dir")
	}

	cases := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
		{"", ""},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, struct{ in, want string }{
			`~\bar`, filepath.Join(home, "bar"),
		})
	}

	for _, tc := range cases {
		got := expandHomePath(tc.in)
		if got != tc.want {
			t.Errorf("expandHomePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestConfigSearchPaths(t *testing.T) {
	paths := configSearchPaths()
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 search paths, got %v", paths)
	}
	if paths[0] != "." {
		t.Fatalf("first path should be '.', got %q", paths[0])
	}
	// /etc/agent must not appear on Windows.
	if runtime.GOOS == "windows" {
		for _, p := range paths {
			if strings.HasPrefix(p, "/etc") || strings.HasPrefix(p, `\etc`) {
				t.Fatalf("windows search paths must not include /etc: %v", paths)
			}
		}
	}
}

func TestParseBashExport(t *testing.T) {
	k, v, ok := parseBashExport(`export FOO="bar baz"`)
	if !ok || k != "FOO" || v != "bar baz" {
		t.Fatalf("parseBashExport failed: ok=%v k=%q v=%q", ok, k, v)
	}
	_, _, ok = parseBashExport("not an export")
	if ok {
		t.Fatal("expected non-export to fail")
	}
}

func TestParsePowerShellEnv(t *testing.T) {
	cases := []struct {
		line     string
		wantKey  string
		wantVal  string
		wantOK   bool
	}{
		{`$env:NVIDIA_API_KEY = "nvapi-abc"`, "NVIDIA_API_KEY", "nvapi-abc", true},
		{`$env:GROQ_API_KEY="gsk_xyz"`, "GROQ_API_KEY", "gsk_xyz", true},
		{`$env:FOO = 'single'`, "FOO", "single", true},
		{`Write-Host "hello"`, "", "", false},
		{`# $env:SKIP = "no"`, "", "", false},
	}
	for _, tc := range cases {
		k, v, ok := parsePowerShellEnv(tc.line)
		if ok != tc.wantOK || k != tc.wantKey || v != tc.wantVal {
			t.Errorf("parsePowerShellEnv(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.line, k, v, ok, tc.wantKey, tc.wantVal, tc.wantOK)
		}
	}
}

func TestParseDotEnv(t *testing.T) {
	k, v, ok := parseDotEnv("NVIDIA_API_KEY=nvapi-123")
	if !ok || k != "NVIDIA_API_KEY" || v != "nvapi-123" {
		t.Fatalf("parseDotEnv failed: ok=%v k=%q v=%q", ok, k, v)
	}
	k, v, ok = parseDotEnv(`export GROQ_API_KEY="gsk_abc"`)
	if !ok || k != "GROQ_API_KEY" || v != "gsk_abc" {
		t.Fatalf("parseDotEnv export failed: ok=%v k=%q v=%q", ok, k, v)
	}
}

func TestDefaultConfigFile(t *testing.T) {
	p := DefaultConfigFile()
	if filepath.Base(p) != "config.yaml" {
		t.Fatalf("DefaultConfigFile base = %q", filepath.Base(p))
	}
	// Ensure parent dir concept is sane.
	if _, err := os.Stat(filepath.Dir(p)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}
}
