package httpserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPServerGracefulShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	})

	server := &http.Server{Handler: handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	var serveWG sync.WaitGroup
	serveWG.Add(1)
	go func() {
		defer serveWG.Done()
		_ = server.Serve(listener)
	}()

	baseURL := "http://" + listener.Addr().String()
	resultCh := make(chan error, 1)
	go func() {
		resp, err := http.Get(baseURL + "/slow")
		if err != nil {
			resultCh <- err
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultCh <- err
			return
		}
		if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "done" {
			resultCh <- io.ErrUnexpectedEOF
			return
		}
		resultCh <- nil
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- server.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	close(release)

	require.NoError(t, <-shutdownDone)
	require.NoError(t, <-resultCh)
	serveWG.Wait()
}
