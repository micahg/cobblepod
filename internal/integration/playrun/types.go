package playrun

// PodcastTitle and PodcastAuthor mirror the constants Cobblepod writes into the
// RSS feed (see internal/podcast/rss.go and internal/processor/processor.go).
const (
	PodcastTitle  = "Playrun Addict Custom Feed"
	PodcastAuthor = "Playrun Addict"
)

// Podcast is the Playrun-side podcast record created/looked-up via
// POST /api/podcast. UUID is assigned by Playrun and required by all
// subsequent calls.
type Podcast struct {
	UUID      string `json:"uuid"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	IsPrivate bool   `json:"isPrivate"`
	URL       string `json:"url"`
}

// upsertPodcastRequest is the body for POST /api/podcast.
type upsertPodcastRequest struct {
	URL       string `json:"url"`
	IsPrivate bool   `json:"isPrivate"`
}

// upsertPodcastResponse is the body for the 201 response from POST /api/podcast.
// The top-level UUID is the podcast UUID used by all subsequent calls.
type upsertPodcastResponse struct {
	UUID string `json:"uuid"`
}

// PodcastDetail represents an episode entry inside GET /api/podcast/{uuid}.
type PodcastDetail struct {
	Title     string          `json:"title"`
	UUID      string          `json:"uuid"`
	Episodes  []PlaylistEntry `json:"episodes"`
	URL       string          `json:"url"`
	IsPrivate bool            `json:"isPrivate"`
}

// PlaylistEntry is the shape used both in GET /api/playlist responses and the
// episode payload nested inside POST /api/playlist/subscribe.
type PlaylistEntry struct {
	UUID        string          `json:"uuid"`
	Title       string          `json:"title"`
	URL         string          `json:"url"`
	Type        string          `json:"type"`
	Podcast     PlaylistPodcast `json:"podcast"`
	PodcastUUID string          `json:"podcast_uuid,omitempty"`
}

// PlaylistPodcast is the nested podcast block that rides along with each
// playlist entry.
type PlaylistPodcast struct {
	UUID      string `json:"uuid"`
	Title     string `json:"title"`
	Author    string `json:"author,omitempty"`
	IsPrivate bool   `json:"isPrivate,omitempty"`
}

// playlistResponse is the body returned by GET /api/playlist and the
// 200/201 responses from POST/DELETE /api/playlist/subscribe.
type playlistResponse struct {
	Playlist []PlaylistEntry `json:"playlist"`
}

// subscribeRequest is the body for POST /api/playlist/subscribe.
type subscribeRequest struct {
	Episode PlaylistEntry `json:"episode"`
}

// unsubscribeRequest is the body for DELETE /api/playlist/subscribe.
type unsubscribeRequest struct {
	UUID        string `json:"uuid"`
	PodcastUUID string `json:"podcast_uuid"`
}
