package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetListTasksPaginatesAndSetsParameters(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "secret" {
			t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
		}
		for _, key := range []string{"include_closed", "subtasks", "include_timl"} {
			if request.URL.Query().Get(key) != "true" {
				t.Errorf("%s ausente", key)
			}
		}
		page := request.URL.Query().Get("page")
		calls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		if page == "0" {
			_, _ = writer.Write([]byte(`{"tasks":[{"id":"a","name":"A","status":{"status":"open","type":"open"}}],"last_page":false}`))
			return
		}
		_, _ = writer.Write([]byte(`{"tasks":[{"id":"b","name":"B","status":{"status":"done","type":"closed"}}],"last_page":true}`))
	}))
	defer server.Close()

	client := NewClientWithOptions("secret", server.URL, server.Client())
	tasks, err := client.GetListTasks(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || calls.Load() != 2 {
		t.Fatalf("tasks=%d calls=%d", len(tasks), calls.Load())
	}
}

func TestGetViewAcceptsNumericParentID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"view":{"id":"v-1","parent":{"id":456,"type":6}}}`))
	}))
	defer server.Close()
	client := NewClientWithOptions("secret", server.URL, server.Client())
	view, err := client.GetView(context.Background(), "v-1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Parent.ID != "456" {
		t.Fatalf("parent ID=%q", view.Parent.ID)
	}
}

func TestHTTPErrorDoesNotLeakToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"ECODE":"OAUTH_019","err":"Token not found"}`))
	}))
	defer server.Close()
	client := NewClientWithOptions("super-secret", server.URL, server.Client())
	client.maxRetries = 0
	_, err := client.GetList(context.Background(), "1")
	if err == nil || !strings.Contains(err.Error(), "OAUTH_019") || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestRequestCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	client := NewClientWithOptions("secret", server.URL, &http.Client{Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetList(ctx, "1")
	if err == nil {
		t.Fatal("se esperaba cancelación")
	}
}
