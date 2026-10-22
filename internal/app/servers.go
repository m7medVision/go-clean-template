package app

import (
	"fmt"
	"os"

	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// server is one listener the app runs, such as the HTTP server.
type server interface {
	Start()
	Notify() <-chan error
	Shutdown() error
}

// namedServer names a server in log lines.
type namedServer struct {
	name   string
	server server
}

// servers are the listeners the app runs. Each transport adds its own, so a transport is wired
// in its own file.
type servers []namedServer

func (s servers) start() {
	for _, n := range s {
		n.server.Start()
	}
}

// waitForShutdown blocks until a signal arrives or any server fails, then shuts every server down.
func (s servers) waitForShutdown(interrupt <-chan os.Signal, l logger.Interface) {
	failed := make(chan error, len(s))

	for _, n := range s {
		go func() {
			if err, ok := <-n.server.Notify(); ok {
				failed <- fmt.Errorf("app - Run - %s.Notify: %w", n.name, err)
			}
		}()
	}

	select {
	case sig := <-interrupt:
		l.Info("app - Run - signal: %s", sig.String())
	case err := <-failed:
		l.Error(err)
	}

	for _, n := range s {
		if err := n.server.Shutdown(); err != nil {
			l.Error(fmt.Errorf("app - Run - %s.Shutdown: %w", n.name, err))
		}
	}
}
