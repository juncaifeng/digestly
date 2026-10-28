package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/juncaifeng/digestly/core/model"
)

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// 故意乱序返回,验证按 index 归位
		_, _ = w.Write([]byte(`{"data":[
			{"index":1,"embedding":[0.2,0.3]},
			{"index":0,"embedding":[0.1,0.0]}
		]}`))
	}))
	defer srv.Close()

	cfg := &model.LLMConfig{Kind: "embedding", BaseURL: srv.URL, Model: "m", APIKey: "k"}
	vecs, err := Embed(context.Background(), cfg, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || vecs[0][0] != 0.1 || vecs[1][1] != 0.3 {
		t.Fatalf("vecs = %v", vecs)
	}
}

func TestEmbedErrors(t *testing.T) {
	if _, err := Embed(context.Background(), &model.LLMConfig{Model: "m"}, []string{"x"}); err == nil {
		t.Fatal("empty base_url should error")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	cfg := &model.LLMConfig{BaseURL: srv.URL, Model: "m"}
	if _, err := Embed(context.Background(), cfg, []string{"x"}); err == nil {
		t.Fatal("count mismatch should error")
	}
}

func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"},{"id":""}]}`))
	}))
	defer srv.Close()

	names, err := ListModels(context.Background(), srv.URL, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "m1" || names[1] != "m2" {
		t.Fatalf("names = %v", names)
	}
	if _, err := ListModels(context.Background(), "", "k"); err == nil {
		t.Fatal("empty base_url should error")
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv2.Close()
	if _, err := ListModels(context.Background(), srv2.URL, ""); err == nil {
		t.Fatal("bad body should error")
	}
}
