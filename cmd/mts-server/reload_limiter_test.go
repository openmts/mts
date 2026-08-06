package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReloadLimiterDoesNotAdmitBeyondLoweredLimit(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.applyLimitState(config{Limits: limitsConfig{MaxConcurrentHTTP: 3}})
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	server := httptest.NewServer(runtime.wrapHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		entered <- struct{}{}
		<-release
	})))
	t.Cleanup(server.Close)

	results := make(chan int, 4)
	for range 3 {
		go requestStatus(server.URL, results)
	}
	for range 3 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("initial request did not enter handler")
		}
	}
	runtime.applyLimitState(config{Limits: limitsConfig{MaxConcurrentHTTP: 1}})
	go requestStatus(server.URL, results)
	select {
	case <-entered:
		close(release)
		t.Fatal("request entered after lowering limit below in-flight count")
	case status := <-results:
		if status != http.StatusTooManyRequests {
			close(release)
			t.Fatalf("status = %d, want 429", status)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("request did not fail fast after lowering limit")
	}
	close(release)
	for range 3 {
		if status := <-results; status != http.StatusOK {
			t.Fatalf("initial request status = %d, want 200", status)
		}
	}
}

func TestReloadLimitStateConcurrentAccess(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.wrapHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	t.Cleanup(server.Close)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for index := range 200 {
			runtime.applyLimitState(config{Limits: limitsConfig{MaxConcurrentHTTP: index%4 + 1}})
		}
	}()
	go func() {
		defer wait.Done()
		client := &http.Client{Timeout: time.Second}
		for range 200 {
			response, err := client.Get(server.URL)
			if err != nil {
				errs <- err
				return
			}
			if _, err := io.Copy(io.Discard, response.Body); err != nil {
				_ = response.Body.Close()
				errs <- err
				return
			}
			if err := response.Body.Close(); err != nil {
				errs <- err
				return
			}
		}
	}()
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent reload request error = %v", err)
	}
}

func requestStatus(url string, result chan<- int) {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		result <- 0
		return
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		_ = response.Body.Close()
		result <- 0
		return
	}
	if err := response.Body.Close(); err != nil {
		result <- 0
		return
	}
	if response.StatusCode == 0 {
		result <- -1
		return
	}
	result <- response.StatusCode
}
