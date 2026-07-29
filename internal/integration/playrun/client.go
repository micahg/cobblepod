// Package playrun is a thin HTTP client for the Playrun watch app API
// (https://www.playrun.app/api/*). It is intentionally minimal: only the
// endpoints Cobblepod uses during finalization (register the custom feed and
// reconcile the watch playlist) are implemented. See
// docs/playrun-playlist-api.md for the captured reference.
package playrun

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BaseURL is the default Playrun API host. Override via NewWithBase for tests.
const BaseURL = "https://www.playrun.app"

// Client is the subset of the Playrun API used by Cobblepod. The interface
// keeps the processor testable without hitting the network.
type Client interface {
	// Validate returns nil if the Playrun JWT is currently accepted.
	// Wraps GET /api/auth/.
	Validate(ctx context.Context) error

	// UpsertPodcast registers the feed at feedURL with Playrun (creating it if
	// needed) and returns the podcast UUID assigned by Playrun. Wraps
	// POST /api/podcast.
	UpsertPodcast(ctx context.Context, feedURL string) (string, error)

	// GetPlaylist returns the episodes currently on the watch playlist.
	// Wraps GET /api/playlist.
	GetPlaylist(ctx context.Context) ([]PlaylistEntry, error)

	// Subscribe adds ep to the watch playlist. Wraps
	// POST /api/playlist/subscribe.
	Subscribe(ctx context.Context, ep PlaylistEntry) error

	// Unsubscribe removes an episode from the watch playlist by episode UUID.
	// Wraps DELETE /api/playlist/subscribe.
	Unsubscribe(ctx context.Context, episodeUUID, podcastUUID string) error
}

// New returns a Client that talks to the default Playrun host using jwt as
// the bearer token.
func New(jwt string) Client {
	return NewWithBase(jwt, BaseURL)
}

// NewWithBase returns a Client pointed at the given base URL (useful for
// tests via httptest.Server).
func NewWithBase(jwt, baseURL string) Client {
	return &client{
		base: strings.TrimRight(baseURL, "/"),
		jwt:  jwt,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// EpisodeUUID returns the Playrun-derived identifier for an episode with the
// given enclosure URL: the lowercase hex SHA1 of the URL bytes. Per
// docs/playrun-playlist-api.md, this must be the *final* enclosure URL written
// to the RSS feed.
func EpisodeUUID(enclosureURL string) string {
	sum := sha1.Sum([]byte(enclosureURL))
	return hex.EncodeToString(sum[:])
}

// MakeEpisode builds the PlaylistEntry shape Cobblepod sends to Playrun for a
// single published episode. podcastUUID is the value returned by UpsertPodcast.
func MakeEpisode(title, enclosureURL, podcastUUID string) PlaylistEntry {
	return PlaylistEntry{
		UUID:        EpisodeUUID(enclosureURL),
		Title:       title,
		URL:         enclosureURL,
		Type:        "mp3",
		Podcast:     PlaylistPodcast{UUID: podcastUUID, Title: PodcastTitle, Author: PodcastAuthor, IsPrivate: true},
		PodcastUUID: podcastUUID,
	}
}

type client struct {
	base string
	jwt  string
	http *http.Client
}

// do performs an HTTP request with the bearer token set, decodes a JSON
// success body into out (if non-nil and status is 2xx), and returns an error
// including the response body on non-2xx responses.
func (c *client) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("playrun: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reqBody)
	if err != nil {
		return fmt.Errorf("playrun: build request: %w", err)
	}
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("playrun: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("playrun: %s %s: status %d: %s", method, path, resp.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("playrun: decode response: %w", err)
		}
	}
	return nil
}

func (c *client) Validate(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/auth/", nil, nil)
}

func (c *client) UpsertPodcast(ctx context.Context, feedURL string) (string, error) {
	var resp upsertPodcastResponse
	if err := c.do(ctx, http.MethodPost, "/api/podcast",
		upsertPodcastRequest{URL: feedURL, IsPrivate: true}, &resp); err != nil {
		return "", err
	}
	if resp.UUID == "" {
		return "", fmt.Errorf("playrun: upsert podcast returned empty uuid")
	}
	return resp.UUID, nil
}

func (c *client) GetPlaylist(ctx context.Context) ([]PlaylistEntry, error) {
	var resp playlistResponse
	if err := c.do(ctx, http.MethodGet, "/api/playlist", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Playlist, nil
}

func (c *client) Subscribe(ctx context.Context, ep PlaylistEntry) error {
	return c.do(ctx, http.MethodPost, "/api/playlist/subscribe",
		subscribeRequest{Episode: ep}, nil)
}

func (c *client) Unsubscribe(ctx context.Context, episodeUUID, podcastUUID string) error {
	return c.do(ctx, http.MethodDelete, "/api/playlist/subscribe",
		unsubscribeRequest{UUID: episodeUUID, PodcastUUID: podcastUUID}, nil)
}
