package podcast

import (
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
	"time"

	"cobblepod/internal/config"
	"cobblepod/internal/gdrive"
)

// RSS represents the root RSS element
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns:itunes,attr"`
	Playrun string   `xml:"xmlns:playrunaddict,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel represents the RSS channel
type Channel struct {
	Title         string   `xml:"title"`
	Description   string   `xml:"description"`
	Link          string   `xml:"link"`
	Language      string   `xml:"language"`
	LastBuildDate string   `xml:"lastBuildDate"`
	Author        string   `xml:"itunes:author"`
	Summary       string   `xml:"itunes:summary"`
	Category      Category `xml:"itunes:category"`
	Explicit      string   `xml:"itunes:explicit"`
	Items         []Item   `xml:"item"`
}

// Category represents iTunes category
type Category struct {
	Text string `xml:"text,attr"`
}

// Item represents an RSS item/episode
type Item struct {
	Title            string    `xml:"title"`
	GUID             GUID      `xml:"guid"`
	OriginalDuration string    `xml:"playrunaddict:originalduration"`
	Enclosure        Enclosure `xml:"enclosure"`
}

// GUID represents the episode GUID
type GUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// Enclosure represents the audio enclosure
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

// RSSProcessor handles RSS feed generation and processing
type RSSProcessor struct {
	channelTitle string
	drive        *gdrive.Service
}

// ProcessedEpisode represents a processed audio episode
type ProcessedEpisode struct {
	Title            string  `json:"title"`
	OriginalURL      string  `json:"original_url,omitempty"`
	OriginalDuration int64   `json:"original_duration"`
	NewDuration      int64   `json:"new_duration"`
	UUID             string  `json:"uuid"`
	Speed            float64 `json:"speed"`
	DownloadURL      string  `json:"download_url,omitempty"`
	OriginalGUID     string  `json:"original_guid,omitempty"`
	TempFile         string  `json:"temp_file,omitempty"`
	DriveFileID      string  `json:"drive_file_id,omitempty"`
}

// ExistingEpisode represents an episode from existing RSS feed or backup data
type ExistingEpisode struct {
	DownloadURL      string `json:"download_url"`
	Length           int64  `json:"length"`
	OriginalDuration int64  `json:"original_duration"`
	OriginalGUID     string `json:"original_guid,omitempty"`
	Offset           int64  `json:"offset,omitempty"` // From PodcastAddict backup
}

// NewRSSProcessor creates a new RSS processor
func NewRSSProcessor(channelTitle string, driveService *gdrive.Service) *RSSProcessor {
	return &RSSProcessor{channelTitle: channelTitle, drive: driveService}
}

// CreateRSSXML generates RSS XML from processed files
func (p *RSSProcessor) CreateRSSXML(processedFiles []ProcessedEpisode) string {
	rss := RSS{
		Version: "2.0",
		Xmlns:   "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Playrun: "http://playrunaddict.com/rss/1.0",
		Channel: Channel{
			Title:         p.channelTitle,
			Description:   "Custom podcast feed generated from processed audio files",
			Link:          "https://example.com",
			Language:      "en-us",
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Author:        "Playrun Addict",
			Summary:       "Custom podcast feed generated from processed audio files",
			Category:      Category{Text: "Technology"},
			Explicit:      "false",
		},
	}

	for _, fileData := range processedFiles {
		item := p.createItemFromFile(fileData)
		rss.Channel.Items = append(rss.Channel.Items, item)
	}

	xmlBytes, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		log.Printf("Error marshaling RSS XML: %v", err)
		return ""
	}
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>%s%s", "\n", string(xmlBytes))
}

func (p *RSSProcessor) createItemFromFile(fileData ProcessedEpisode) Item {
	title := fileData.Title
	guid := fileData.OriginalGUID
	if guid == "" {
		if fileData.UUID != "" {
			guid = fileData.UUID
		} else {
			guid = fmt.Sprintf("episode-%d", hashString(title))
		}
	}
	originalDuration := fileData.OriginalDuration
	newDuration := fileData.NewDuration
	downloadURL := fileData.DownloadURL
	if downloadURL == "" {
		if driveFileID := fileData.DriveFileID; driveFileID != "" {
			downloadURL = p.drive.GenerateDownloadURL(driveFileID)
		}
	}
	return Item{
		Title:            title,
		GUID:             GUID{IsPermaLink: "false", Value: guid},
		OriginalDuration: strconv.FormatInt(originalDuration, 10),
		Enclosure:        Enclosure{URL: downloadURL, Type: "audio/mpeg", Length: strconv.FormatInt(newDuration, 10)},
	}
}

// GetRSSFeedID gets the RSS feed file ID from Google Drive
func (p *RSSProcessor) GetRSSFeedID() string {
	files, err := p.drive.GetFiles(config.RSSQuery, true)
	if err != nil {
		log.Printf("Error searching for RSS feed: %v", err)
		return ""
	}
	if len(files) == 0 {
		return ""
	}
	return files[0].Id
}

// ExtractEpisodeMapping extracts episode mapping from RSS content
func (p *RSSProcessor) ExtractEpisodeMapping(xmlContent string) (map[string]ExistingEpisode, error) {
	var rss RSS
	if err := xml.Unmarshal([]byte(xmlContent), &rss); err != nil {
		return nil, fmt.Errorf("failed to parse RSS XML: %w", err)
	}

	episodeMapping := make(map[string]ExistingEpisode)
	for _, item := range rss.Channel.Items {
		title := item.Title
		if title == "" {
			title = "Untitled Episode"
		}

		originalDuration, _ := strconv.ParseInt(item.OriginalDuration, 10, 64)
		length, _ := strconv.ParseInt(item.Enclosure.Length, 10, 64)

		episode := ExistingEpisode{
			DownloadURL:      item.Enclosure.URL,
			Length:           length,
			OriginalDuration: originalDuration,
			OriginalGUID:     item.GUID.Value,
		}

		episodeMapping[title] = episode
	}
	return episodeMapping, nil
}

func hashString(s string) int {
	hash := 0
	for _, char := range s {
		hash = hash*31 + int(char)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
