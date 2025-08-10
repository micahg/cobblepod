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

// NewRSSProcessor creates a new RSS processor
func NewRSSProcessor(channelTitle string, driveService *gdrive.Service) *RSSProcessor {
	return &RSSProcessor{channelTitle: channelTitle, drive: driveService}
}

// CreateRSSXML generates RSS XML from processed files
func (p *RSSProcessor) CreateRSSXML(processedFiles []map[string]interface{}) string {
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

func (p *RSSProcessor) createItemFromFile(fileData map[string]interface{}) Item {
	title := getStringFromMap(fileData, "title", "Untitled Episode")
	guid := getStringFromMap(fileData, "original_guid", "")
	if guid == "" {
		if uuid := getStringFromMap(fileData, "uuid", ""); uuid != "" {
			guid = uuid
		} else {
			guid = fmt.Sprintf("episode-%d", hashString(title))
		}
	}
	originalDuration := getIntFromMap(fileData, "original_duration", 0)
	newDuration := getIntFromMap(fileData, "new_duration", 0)
	downloadURL := getStringFromMap(fileData, "download_url", "")
	if downloadURL == "" {
		if driveFileID := getStringFromMap(fileData, "drive_file_id", ""); driveFileID != "" {
			downloadURL = p.drive.GenerateDownloadURL(driveFileID)
		}
	}
	return Item{
		Title:            title,
		GUID:             GUID{IsPermaLink: "false", Value: guid},
		OriginalDuration: strconv.Itoa(originalDuration),
		Enclosure:        Enclosure{URL: downloadURL, Type: "audio/mpeg", Length: strconv.Itoa(newDuration)},
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
func (p *RSSProcessor) ExtractEpisodeMapping(xmlContent string) (map[string]map[string]interface{}, error) {
	var rss RSS
	if err := xml.Unmarshal([]byte(xmlContent), &rss); err != nil {
		return nil, fmt.Errorf("failed to parse RSS XML: %w", err)
	}
	episodeMapping := make(map[string]map[string]interface{})
	for _, item := range rss.Channel.Items {
		title := item.Title
		if title == "" {
			title = "Untitled Episode"
		}
		originalDuration, _ := strconv.Atoi(item.OriginalDuration)
		length, _ := strconv.Atoi(item.Enclosure.Length)
		episodeData := map[string]interface{}{
			"download_url":      item.Enclosure.URL,
			"length":            length,
			"original_duration": originalDuration,
		}
		if item.GUID.Value != "" {
			episodeData["original_guid"] = item.GUID.Value
		}
		episodeMapping[title] = episodeData
	}
	return episodeMapping, nil
}

// Helper functions
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}
func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultValue
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
