package playrun

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recordingServer wraps httptest.Server to record request bodies and emit
// configurable responses. Each handler runs once and the next call uses the
// next entry in responses (or repeats the last).
type recordingServer struct {
	*httptest.Server
	mux      *http.ServeMux
	requests []recRequest
}

type recRequest struct {
	method string
	path   string
	body   string
	auth   string
}

func newRecordingServer() *recordingServer {
	mux := http.NewServeMux()
	srv := &recordingServer{mux: mux}
	srv.Server = httptest.NewServer(mux)
	return srv
}

func (r *recordingServer) handle(path string, fn func(w http.ResponseWriter, req *http.Request)) {
	r.mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.requests = append(r.requests, recRequest{
			method: req.Method,
			path:   req.URL.Path,
			body:   string(body),
			auth:   req.Header.Get("Authorization"),
		})
		fn(w, req)
	})
}

func TestEpisodeUUID(t *testing.T) {
	// Verified against the HAR sample in docs/playrun-playlist-api.md.
	url := "https://drive.usercontent.google.com/download?id=1K6mUPanlLUfYne8jZW5ocssGPp-SkoMl&export=download&authuser=0&confirm=t"
	want := "04d9b43a7b387e0b81e9cf848063a25b33aa7cee"
	if got := EpisodeUUID(url); got != want {
		t.Errorf("EpisodeUUID = %q, want %q", got, want)
	}
}

func TestMakeEpisode(t *testing.T) {
	ep := MakeEpisode("Critical Role - C2E35", "https://example.com/file.mp3", "pod-uuid-123")
	if ep.UUID != EpisodeUUID("https://example.com/file.mp3") {
		t.Errorf("expected UUID to be sha1(url), got %q", ep.UUID)
	}
	if ep.Title != "Critical Role - C2E35" || ep.Type != "mp3" {
		t.Errorf("unexpected episode: %+v", ep)
	}
	if ep.Podcast.UUID != "pod-uuid-123" || ep.Podcast.Title != PodcastTitle {
		t.Errorf("unexpected podcast block: %+v", ep.Podcast)
	}
	if ep.PodcastUUID != "pod-uuid-123" {
		t.Errorf("expected PodcastUUID to be set, got %q", ep.PodcastUUID)
	}
}

func TestClient_Validate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newRecordingServer()
		defer srv.Server.Close()
		srv.handle("/api/auth/", func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", req.Method)
			}
			w.WriteHeader(http.StatusOK)
		})

		c := NewWithBase("tok", srv.Server.URL)
		if err := c.Validate(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := srv.requests[0].auth; got != "Bearer tok" {
			t.Errorf("expected Authorization Bearer tok, got %q", got)
		}
	})

	t.Run("auth failure", func(t *testing.T) {
		srv := newRecordingServer()
		defer srv.Server.Close()
		srv.handle("/api/auth/", func(w http.ResponseWriter, req *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})

		c := NewWithBase("bad", srv.Server.URL)
		err := c.Validate(context.Background())
		if err == nil || !strings.Contains(err.Error(), "status 401") {
			t.Errorf("expected status 401 error, got %v", err)
		}
	})
}

func TestClient_UpsertPodcast(t *testing.T) {
	t.Run("success returns uuid", func(t *testing.T) {
		srv := newRecordingServer()
		defer srv.Server.Close()
		srv.handle("/api/podcast", func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", req.Method)
			}
			var body upsertPodcastRequest
			if err := json.Unmarshal([]byte(srv.requests[len(srv.requests)-1].body), &body); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if body.URL != "https://feed.example/rss" || !body.IsPrivate {
				t.Errorf("unexpected request body: %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":"pod-abc-123"}`))
		})

		c := NewWithBase("tok", srv.Server.URL)
		uuid, err := c.UpsertPodcast(context.Background(), "https://feed.example/rss")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uuid != "pod-abc-123" {
			t.Errorf("expected pod-abc-123, got %q", uuid)
		}
	})

	t.Run("empty uuid is an error", func(t *testing.T) {
		srv := newRecordingServer()
		defer srv.Server.Close()
		srv.handle("/api/podcast", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":""}`))
		})

		c := NewWithBase("tok", srv.Server.URL)
		_, err := c.UpsertPodcast(context.Background(), "https://feed.example/rss")
		if err == nil || !strings.Contains(err.Error(), "empty uuid") {
			t.Errorf("expected empty uuid error, got %v", err)
		}
	})
}

func TestClient_GetPlaylist(t *testing.T) {
	srv := newRecordingServer()
	defer srv.Server.Close()
	srv.handle("/api/playlist", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"playlist":[{"uuid":"ep1","title":"Ep 1","url":"https://x/1","type":"mp3","podcast":{"uuid":"pod-abc"}}]}`))
	})

	c := NewWithBase("tok", srv.Server.URL)
	playlist, err := c.GetPlaylist(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(playlist) != 1 || playlist[0].UUID != "ep1" || playlist[0].Podcast.UUID != "pod-abc" {
		t.Errorf("unexpected playlist: %+v", playlist)
	}
}

func TestClient_Subscribe(t *testing.T) {
	srv := newRecordingServer()
	defer srv.Server.Close()
	srv.handle("/api/playlist/subscribe", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", req.Method)
		}
		var body subscribeRequest
		if err := json.Unmarshal([]byte(srv.requests[len(srv.requests)-1].body), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.Episode.UUID != EpisodeUUID("https://x/1") {
			t.Errorf("expected UUID to be sha1(url), got %q", body.Episode.UUID)
		}
		if body.Episode.PodcastUUID != "pod-uuid" {
			t.Errorf("expected PodcastUUID pod-uuid, got %q", body.Episode.PodcastUUID)
		}
		w.WriteHeader(http.StatusCreated)
	})

	c := NewWithBase("tok", srv.Server.URL)
	ep := MakeEpisode("Title", "https://x/1", "pod-uuid")
	if err := c.Subscribe(context.Background(), ep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Unsubscribe(t *testing.T) {
	srv := newRecordingServer()
	defer srv.Server.Close()
	srv.handle("/api/playlist/subscribe", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", req.Method)
		}
		var body unsubscribeRequest
		if err := json.Unmarshal([]byte(srv.requests[len(srv.requests)-1].body), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body.UUID != "ep-uuid" || body.PodcastUUID != "pod-uuid" {
			t.Errorf("unexpected unsubscribe body: %+v", body)
		}
		w.WriteHeader(http.StatusOK)
	})

	c := NewWithBase("tok", srv.Server.URL)
	if err := c.Unsubscribe(context.Background(), "ep-uuid", "pod-uuid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := newRecordingServer()
	defer srv.Server.Close()
	srv.handle("/api/playlist", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	c := NewWithBase("tok", srv.Server.URL)
	_, err := c.GetPlaylist(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 'status 500' in error, got %v", err)
	}
}
