package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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
		{"invalid letters in hostport", "myapp:abc:80", false},
		{"valid named podport", "myapp:8080:http", true},
		{"valid named podport with hyphen", "myapp:8080:my-port", true},
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
	kubectx = ""
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

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"numeric", "8080", true},
		{"zero", "0", true},
		{"empty", "", false},
		{"letters", "http", false},
		{"mixed", "80a", false},
		{"hyphen", "my-port", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNumeric(tt.s); got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestResolveNamedPort(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		prev := execCommand
		execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "-n", "3000")
		}
		defer func() { execCommand = prev }()

		port, err := resolveNamedPort(context.Background(), "mypod", "http")
		if err != nil {
			t.Fatal(err)
		}
		if port != "3000" {
			t.Errorf("got %q, want %q", port, "3000")
		}
	})

	t.Run("not found", func(t *testing.T) {
		prev := execCommand
		execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "-n", "")
		}
		defer func() { execCommand = prev }()

		_, err := resolveNamedPort(context.Background(), "mypod", "nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown port name")
		}
	})

	t.Run("command failure", func(t *testing.T) {
		prev := execCommand
		execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false")
		}
		defer func() { execCommand = prev }()

		_, err := resolveNamedPort(context.Background(), "nonexistent", "http")
		if err == nil {
			t.Fatal("expected error on command failure")
		}
	})
}

func TestStartForward_WithNamedPort(t *testing.T) {
	defer discardStdout()()
	kubectx = ""

	var callIndex int
	var mu sync.Mutex
	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "kubectl" {
			return exec.Command("true")
		}
		mu.Lock()
		callIndex++
		index := callIndex
		mu.Unlock()

		if len(args) > 0 && args[0] == "get" && index == 1 {
			// First get: locate pod
			return exec.Command("echo", "-n", "mypod")
		}
		if len(args) > 0 && args[0] == "get" && index == 2 {
			// Second get: resolve named port
			return exec.Command("echo", "-n", "3000")
		}
		if len(args) > 0 && args[0] == "port-forward" {
			return exec.CommandContext(ctx, "sleep", "30")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = prev }()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "testapp", "8080", "http", &wg)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit")
	}
}

func TestStartForward_RetryOnPortForwardFailure(t *testing.T) {
	defer discardStdout()()
	kubectx = ""

	var mu sync.Mutex
	var getCount, pfCount int
	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "kubectl" {
			return exec.Command("true")
		}
		mu.Lock()
		defer mu.Unlock()
		if len(args) > 0 && args[0] == "get" {
			getCount++
			return exec.Command("echo", "-n", "mypod")
		}
		if len(args) > 0 && args[0] == "port-forward" {
			pfCount++
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = prev }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "testapp", "8080", "80", &wg)
		close(done)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startForward did not exit")
	}

	mu.Lock()
	t.Logf("get calls: %d, port-forward calls: %d", getCount, pfCount)
	mu.Unlock()
	if getCount < 2 {
		t.Errorf("expected at least 2 getPodName calls, got %d", getCount)
	}
	if pfCount < 2 {
		t.Errorf("expected at least 2 port-forward calls, got %d", pfCount)
	}
}

func TestStartForward_PortForwardSuccessThenFail(t *testing.T) {
	defer discardStdout()()

	callCount := 0
	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if len(args) > 0 && args[0] == "get" {
			return exec.Command("echo", "-n", "mypod")
		}
		// First port-forward succeeds, second fails
		if callCount <= 3 {
			return exec.Command("echo", "-n", "ok")
		}
		return exec.Command("false")
	}
	defer func() { execCommand = prev }()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "testapp", "8080", "80", &wg)
		close(done)
	}()

	time.Sleep(250 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit")
	}

	if callCount < 4 {
		t.Errorf("expected multiple call cycles (success then fail), got %d total calls", callCount)
	}
}

func TestStartForward_VerboseMode(t *testing.T) {
	defer discardStdout()()

	resetFlags()
	flag.Bool("verbose", false, "")
	os.Args = []string{"kubeforward", "--verbose", "myapp:8080:80"}
	flag.Parse()
	defer resetFlags()

	var captured *bytes.Buffer
	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "get" {
			return exec.Command("echo", "-n", "mypod")
		}
		cmd := exec.CommandContext(ctx, "echo", "-n", "forwarding ok")
		buf := new(bytes.Buffer)
		captured = buf
		cmd.Stdout = buf
		cmd.Stderr = buf
		return cmd
	}
	defer func() { execCommand = prev }()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "testapp", "8080", "80", &wg)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit")
	}

	if captured == nil {
		t.Error("expected stdout/stderr capture in verbose mode")
	}
}

func TestStartForward_QuietMode(t *testing.T) {
	resetFlags()
	flag.Bool("quiet", false, "")
	os.Args = []string{"kubeforward", "--quiet", "myapp:8080:80"}
	flag.Parse()
	defer resetFlags()

	defer discardStdout()()

	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "get" {
			return exec.Command("echo", "-n", "mypod")
		}
		return exec.Command("true")
	}
	defer func() { execCommand = prev }()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	go func() {
		startForward(ctx, "testapp", "8080", "80", &wg)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startForward did not exit")
	}
}

func TestGetPodNameWithContextFlag(t *testing.T) {
	kubectx = "my-cluster"
	defer func() { kubectx = "" }()

	prev := execCommand
	execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "--context" && args[1] == "my-cluster" {
			return exec.Command("echo", "-n", "mypod")
		}
		t.Errorf("expected --context my-cluster in args, got %v", args)
		return exec.Command("false")
	}
	defer func() { execCommand = prev }()

	name, err := getPodName(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if name != "mypod" {
		t.Errorf("got %q, want %q", name, "mypod")
	}
}

func TestMainSuccess(t *testing.T) {
	defer discardStdout()()

	os.Args = []string{"kubeforward", "myapp:8080:80"}

	resetFlags()

	prev := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "get" {
			return exec.Command("echo", "-n", "mypod")
		}
		return exec.CommandContext(ctx, "sleep", "30")
	}
	defer func() { execCommand = prev }()

	done := make(chan struct{})
	go func() {
		defer func() { recover() }()
		main()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGINT)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("main did not exit after SIGINT")
	}
}

func BenchmarkValidDeployInfo(b *testing.B) {
	for range b.N {
		ValidDeployInfo("myapp:8080:80")
	}
}
