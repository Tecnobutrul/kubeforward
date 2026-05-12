package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	kubectx = ""
}

func discardStdout() func() {
	old := os.Stdout
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = devNull
	return func() {
		devNull.Close()
		os.Stdout = old
	}
}

func TestValidDeployInfo(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"valid basic", "myapp:8080:80", true},
		{"valid with underscores", "my_app:3000:3000", true},
		{"valid with dots", "my.app:443:8443", true},
		{"valid with numbers in name", "app1:3000:3000", true},
		{"valid minimal two-char name", "ab:1:1", true},
		{"valid max port 65535", "myapp:65535:65535", true},
		{"valid port 0", "myapp:0:0", true},
		{"invalid no hostport", "myapp::80", false},
		{"invalid letters in port", "myapp:abc:80", false},
		{"invalid empty string", "", false},
		{"invalid special chars", "my@app:8080:80", false},
		{"invalid leading colon", ":8080:80", false},
		{"invalid name only", "myapp", false},
		{"invalid hyphen in name", "my-app:8080:80", false},
		{"name starting with number is valid per regex", "1app:8080:80", true},
	}

	// Max name length (1 + 250 chars)
	maxName := "a" + strings.Repeat("b", 250)
	tests = append(tests, struct {
		name string
		arg  string
		want bool
	}{"valid max name length", maxName + ":8080:80", true})

	// Exceed max name length (252 chars total)
	longName := "a" + strings.Repeat("b", 251)
	tests = append(tests, struct {
		name string
		arg  string
		want bool
	}{"invalid name too long", longName + ":8080:80", false})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidDeployInfo(tt.arg); got != tt.want {
				t.Errorf("ValidDeployInfo(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestGetArgsConfig(t *testing.T) {
	tests := []struct {
		name     string
		initial  []Deployment
		args     []string
		want     []Deployment
		wantSkip int
	}{
		{
			name:     "add new deployment",
			initial:  []Deployment{},
			args:     []string{"myapp:8080:80"},
			want:     []Deployment{{Name: "myapp", Hostport: "8080", Podport: "80"}},
		},
		{
			name:     "add multiple deployments",
			initial:  []Deployment{},
			args:     []string{"app1:3000:3000", "app2:4000:4000"},
			want: []Deployment{
				{Name: "app1", Hostport: "3000", Podport: "3000"},
				{Name: "app2", Hostport: "4000", Podport: "4000"},
			},
		},
		{
			name:     "overwrite existing",
			initial:  []Deployment{{Name: "myapp", Hostport: "8080", Podport: "80"}},
			args:     []string{"myapp:9090:90"},
			want:     []Deployment{{Name: "myapp", Hostport: "9090", Podport: "90"}},
		},
		{
			name: "skip invalid format",
			initial:  []Deployment{},
			args:     []string{"invalid", "validapp:80:80"},
			want:     []Deployment{{Name: "validapp", Hostport: "80", Podport: "80"}},
		},
		{
			name:     "overwrite preserves order",
			initial: []Deployment{
				{Name: "app1", Hostport: "3000", Podport: "3000"},
				{Name: "app2", Hostport: "4000", Podport: "4000"},
			},
			args: []string{"app1:5000:5000"},
			want: []Deployment{
				{Name: "app1", Hostport: "5000", Podport: "5000"},
				{Name: "app2", Hostport: "4000", Podport: "4000"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer discardStdout()()
			config := &Yaml{Deployment: tt.initial}
			getArgsConfig(config, tt.args)

			if len(config.Deployment) != len(tt.want) {
				t.Fatalf("got %d deployments, want %d\n%+v", len(config.Deployment), len(tt.want), config.Deployment)
			}

			for i, dp := range config.Deployment {
				if dp != tt.want[i] {
					t.Errorf("deployment[%d] = %+v, want %+v", i, dp, tt.want[i])
				}
			}
		})
	}
}

func TestGetConfFile(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		content := []byte("deployment:\n  - name: myapp\n    hostport: \"8080\"\n    podport: \"80\"\n")
		tmpFile, err := os.CreateTemp(t.TempDir(), "*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpFile.Write(content); err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()

		config, err := getConfFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		if len(config.Deployment) != 1 {
			t.Fatalf("got %d deployments, want 1", len(config.Deployment))
		}
		if config.Deployment[0].Name != "myapp" {
			t.Errorf("name = %q, want %q", config.Deployment[0].Name, "myapp")
		}
		if config.Deployment[0].Hostport != "8080" {
			t.Errorf("hostport = %q, want %q", config.Deployment[0].Hostport, "8080")
		}
		if config.Deployment[0].Podport != "80" {
			t.Errorf("podport = %q, want %q", config.Deployment[0].Podport, "80")
		}
	})

	t.Run("empty yaml", func(t *testing.T) {
		content := []byte("deployment: []\n")
		tmpFile, err := os.CreateTemp(t.TempDir(), "*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpFile.Write(content); err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()

		config, err := getConfFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		if config.Deployment == nil {
			t.Fatal("Deployment should not be nil")
		}
		if len(config.Deployment) != 0 {
			t.Errorf("got %d deployments, want 0", len(config.Deployment))
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := getConfFile("/nonexistent/file.yaml")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		content := []byte("{invalid: [yaml\n")
		tmpFile, err := os.CreateTemp(t.TempDir(), "*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpFile.Write(content); err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()

		_, err = getConfFile(tmpFile.Name())
		if err == nil {
			t.Fatal("expected error for invalid yaml")
		}
	})
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "testfile")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	tmpDir := t.TempDir()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", tmpFile.Name(), true},
		{"non-existing file", "/nonexistent/path/that/does/not/exist", false},
		{"directory", tmpDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileExists(tt.path); got != tt.want {
				t.Errorf("fileExists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetPodName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "-n", "mypod")
		}
		defer func() { execCommand = exec.CommandContext }()

		name, err := getPodName(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}
		if name != "mypod" {
			t.Errorf("got %q, want %q", name, "mypod")
		}
	})

	t.Run("command failure", func(t *testing.T) {
		execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false")
		}
		defer func() { execCommand = exec.CommandContext }()

		_, err := getPodName(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sleep", "10")
		}
		defer func() { execCommand = exec.CommandContext }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := getPodName(ctx, "test")
		if err != context.Canceled {
			t.Errorf("got %v, want context.Canceled", err)
		}
	})
}

func TestStartForward_GetPodNameFailure(t *testing.T) {
	defer discardStdout()()

	execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "nonexistent", "8080", "80", &wg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit after getPodName failure")
	}
}

func TestStartForward_ContextCancelledWhileRunning(t *testing.T) {
	defer discardStdout()()

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "10")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "test", "8080", "80", &wg)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit after context cancellation")
	}
}

func TestStartForward_ContextCancelledBeforeStart(t *testing.T) {
	execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		t.Error("execCommand should not be called")
		return exec.Command("true")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "test", "8080", "80", &wg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit immediately with cancelled context")
	}
}

func TestIsFlagPassed(t *testing.T) {
	t.Run("flag not set", func(t *testing.T) {
		if isFlagPassed("nonexistent") {
			t.Error("expected false for flag that was never set")
		}
	})
}

func TestDeploymentStruct(t *testing.T) {
	d := Deployment{Name: "test", Hostport: "8080", Podport: "80"}
	if d.Name != "test" || d.Hostport != "8080" || d.Podport != "80" {
		t.Errorf("Deployment struct fields do not match: %+v", d)
	}
}

func TestArgInfo(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"kubeforward"}
		path, args := argInfo()
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
		if path != "deploy.yaml" {
			t.Errorf("path = %q, want %q", path, "deploy.yaml")
		}
	})

	t.Run("custom file path", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"kubeforward", "--file=/tmp/test.yaml"}
		path, args := argInfo()
		want, _ := filepath.Abs("/tmp/test.yaml")
		if path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("with context flag", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"kubeforward", "--context", "my-cluster", "myapp:8080:80"}
		path, args := argInfo()
		if kubectx != "my-cluster" {
			t.Errorf("kubectx = %q, want %q", kubectx, "my-cluster")
		}
		if len(args) != 1 || args[0] != "myapp:8080:80" {
			t.Errorf("args = %v, want [myapp:8080:80]", args)
		}
		if path != "deploy.yaml" {
			t.Errorf("path = %q, want %q", path, "deploy.yaml")
		}
	})

	t.Run("with positional args only", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"kubeforward", "app1:3000:3000", "app2:4000:4000"}
		path, args := argInfo()
		want := []string{"app1:3000:3000", "app2:4000:4000"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range args {
			if args[i] != want[i] {
				t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
			}
		}
		if path != "deploy.yaml" {
			t.Errorf("path = %q, want %q", path, "deploy.yaml")
		}
	})

	t.Run("all flags combined", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"kubeforward", "--context", "prod", "--file=/etc/kube/forward.yaml", "svc:8080:80"}
		path, args := argInfo()
		if kubectx != "prod" {
			t.Errorf("kubectx = %q, want %q", kubectx, "prod")
		}
		wantPath, _ := filepath.Abs("/etc/kube/forward.yaml")
		if path != wantPath {
			t.Errorf("path = %q, want %q", path, wantPath)
		}
		if len(args) != 1 || args[0] != "svc:8080:80" {
			t.Errorf("args = %v, want [svc:8080:80]", args)
		}
	})
}

func BenchmarkValidDeployInfo(b *testing.B) {
	for range b.N {
		ValidDeployInfo("myapp:8080:80")
	}
}
