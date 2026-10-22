package app

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/logger"
)

var errListen = errors.New("listen failed")

type fakeServer struct {
	mu       sync.Mutex
	notify   chan error
	started  bool
	shutdown bool
}

func newFakeServer() *fakeServer {
	return &fakeServer{notify: make(chan error, 1)}
}

func (f *fakeServer) Start() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.started = true
}

func (f *fakeServer) Notify() <-chan error { return f.notify }

func (f *fakeServer) Shutdown() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.shutdown = true

	return nil
}

func (f *fakeServer) state() (started, shutdown bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.started, f.shutdown
}

func TestServersStartStartsEveryServer(t *testing.T) {
	t.Parallel()

	a, b := newFakeServer(), newFakeServer()
	s := servers{{name: "a", server: a}, {name: "b", server: b}}

	s.start()

	for name, f := range map[string]*fakeServer{"a": a, "b": b} {
		if started, _ := f.state(); !started {
			t.Errorf("server %s was not started", name)
		}
	}
}

func TestServersWaitShutsAllDownWhenOneFails(t *testing.T) {
	t.Parallel()

	a, b := newFakeServer(), newFakeServer()
	s := servers{{name: "a", server: a}, {name: "b", server: b}}

	b.notify <- errListen

	s.waitForShutdown(make(chan os.Signal), logger.New("error"))

	for name, f := range map[string]*fakeServer{"a": a, "b": b} {
		if _, shutdown := f.state(); !shutdown {
			t.Errorf("server %s was not shut down", name)
		}
	}
}

func TestServersWaitShutsAllDownOnSignal(t *testing.T) {
	t.Parallel()

	a := newFakeServer()
	s := servers{{name: "a", server: a}}

	interrupt := make(chan os.Signal, 1)
	interrupt <- syscall.SIGTERM

	s.waitForShutdown(interrupt, logger.New("error"))

	if _, shutdown := a.state(); !shutdown {
		t.Error("server was not shut down")
	}
}
