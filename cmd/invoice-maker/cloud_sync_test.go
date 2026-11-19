package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/cloud"
)

func TestPushLocalCloudChangesPersistsCursorAndTombstone(t *testing.T) {
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			fmt.Fprint(w, `{"token":"session"}`)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/cloud/documents/"):
			if r.Header.Get("Authorization") != "Bearer session" {
				t.Error("missing bearer token")
			}
			var body struct {
				ID      string          `json:"id"`
				Kind    string          `json:"kind"`
				Version int64           `json:"version"`
				Deleted bool            `json:"deleted"`
				Data    json.RawMessage `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			body.ID = strings.TrimPrefix(r.URL.Path, "/api/v1/cloud/documents/")
			if body.ID == "" || body.Kind != "customer" {
				t.Errorf("unexpected document: %+v", body)
			}
			if puts == 0 && (body.Deleted || body.Version != 0) {
				t.Errorf("initial document: %+v", body)
			}
			if puts == 1 && (!body.Deleted || body.Version != 1) {
				t.Errorf("tombstone: %+v", body)
			}
			puts++
			fmt.Fprintf(w, `{"id":%q,"kind":"customer","version":%d,"updated_at":100,"deleted":%t,"data":{}}`, body.ID, body.Version+1, body.Deleted)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "push-sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.DB().ExecContext(ctx, "INSERT INTO customers(name,phone,created_at,updated_at) VALUES('Local customer','09120000000','now','now')"); err != nil {
		t.Fatal(err)
	}
	client, err := cloud.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Login(ctx, "user@example.test", "password"); err != nil {
		t.Fatal(err)
	}
	cursor, pushed, conflicts, err := pushLocalCloudChanges(ctx, store, client, 0)
	if err != nil || pushed != 1 || cursor == 0 || puts != 1 {
		t.Fatalf("cursor=%d pushed=%d conflicts=%d puts=%d err=%v", cursor, pushed, conflicts, puts, err)
	}
	cursor2, pushed, conflicts, err := pushLocalCloudChanges(ctx, store, client, cursor)
	if err != nil || pushed != 0 || cursor2 != cursor {
		t.Fatalf("repeat cursor=%d pushed=%d err=%v", cursor2, pushed, err)
	}
	if _, err = store.DB().ExecContext(ctx, "DELETE FROM customers"); err != nil {
		t.Fatal(err)
	}
	cursor, pushed, conflicts, err = pushLocalCloudChanges(ctx, store, client, cursor)
	if err != nil || pushed != 1 || cursor <= cursor2 || puts != 2 {
		t.Fatalf("delete cursor=%d pushed=%d puts=%d err=%v", cursor, pushed, puts, err)
	}
}
