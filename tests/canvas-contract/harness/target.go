package harness

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	Binary       string
	Arguments    []string
	Directory    string
	Environment  []string
	Origin       string
	HealthPath   string
	Output       io.Writer
	ReadyTimeout time.Duration
	StopTimeout  time.Duration
}

type Target struct {
	mutex      sync.Mutex
	config     Config
	process    *exec.Cmd
	exit       chan error
	generation int
}

func LocalOrigin(raw string) (*url.URL, error) {
	origin, err := url.Parse(raw)
	if err != nil || origin.Scheme != "http" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" {
		return nil, errors.New("target control requires an HTTP loopback origin without path or credentials")
	}
	if origin.Hostname() != "127.0.0.1" && origin.Hostname() != "::1" || origin.Port() == "" {
		return nil, errors.New("target control requires an explicit loopback IP and port")
	}
	return origin, nil
}

func NewTarget(config Config) (*Target, error) {
	if _, err := LocalOrigin(config.Origin); err != nil {
		return nil, err
	}
	if config.Binary == "" || config.Output == nil || config.ReadyTimeout <= 0 {
		return nil, errors.New("target binary, output and readiness deadline are required")
	}
	if config.HealthPath == "" {
		config.HealthPath = "/api/health"
	}
	if config.StopTimeout == 0 {
		config.StopTimeout = 5 * time.Second
	}
	if config.StopTimeout < 0 {
		return nil, errors.New("target stop deadline must be positive")
	}
	if config.HealthPath[0] != '/' || len(config.HealthPath) > 1 && config.HealthPath[1] == '/' {
		return nil, errors.New("health path must be an origin-relative path")
	}
	return &Target{config: config}, nil
}

func (target *Target) Start(ctx context.Context) error {
	target.mutex.Lock()
	defer target.mutex.Unlock()
	return target.start(ctx)
}

func (target *Target) start(ctx context.Context) error {
	if target.process != nil {
		return errors.New("target already started")
	}
	origin, _ := LocalOrigin(target.config.Origin)
	connection, err := net.DialTimeout("tcp", origin.Host, 300*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		return errors.New("target port occupied; refusing to adopt or terminate another process")
	}
	process := exec.Command(target.config.Binary, target.config.Arguments...)
	process.Dir = target.config.Directory
	process.Env = target.config.Environment
	process.Stdout = target.config.Output
	process.Stderr = target.config.Output
	if err := process.Start(); err != nil {
		return errors.New("target failed to start")
	}
	target.process = process
	target.exit = make(chan error, 1)
	exit := target.exit
	go func() { exit <- process.Wait() }()
	client := &http.Client{
		Timeout:       time.Second,
		Transport:     &http.Transport{Proxy: nil},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	readyContext, cancel := context.WithTimeout(ctx, target.config.ReadyTimeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-readyContext.Done():
			target.stop(true)
			return errors.New("target readiness deadline or cancellation reached")
		case <-target.exit:
			target.process = nil
			return errors.New("target exited before readiness")
		case <-ticker.C:
			request, err := http.NewRequestWithContext(readyContext, http.MethodGet, target.config.Origin+target.config.HealthPath, nil)
			if err != nil {
				target.stop(true)
				return errors.New("invalid target health request")
			}
			response, err := client.Do(request)
			if err != nil {
				continue
			}
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				target.generation++
				return nil
			}
		}
	}
}

func (target *Target) stop(crash bool) error {
	if target.process == nil {
		return nil
	}
	if crash {
		_ = target.process.Process.Kill()
	} else {
		_ = target.process.Process.Signal(syscall.SIGTERM)
	}
	select {
	case <-target.exit:
	case <-time.After(target.config.StopTimeout):
		_ = target.process.Process.Kill()
		<-target.exit
		target.process = nil
		return errors.New("target shutdown deadline exceeded; forced cleanup is not a successful graceful restart")
	}
	target.process = nil
	return nil
}

func (target *Target) Stop() {
	target.mutex.Lock()
	defer target.mutex.Unlock()
	target.stop(false)
}

func (target *Target) Restart(ctx context.Context, mode string) (int, error) {
	target.mutex.Lock()
	defer target.mutex.Unlock()
	if mode != "crash" && mode != "graceful" {
		return 0, errors.New("unknown restart mode")
	}
	if target.process == nil {
		return 0, errors.New("no owned target process to restart")
	}
	if err := target.stop(mode == "crash"); err != nil {
		return 0, err
	}
	if err := target.start(ctx); err != nil {
		return 0, err
	}
	return target.generation, nil
}
