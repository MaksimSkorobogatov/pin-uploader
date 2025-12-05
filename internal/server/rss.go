package server

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
)

// RSS represents the RSS feed root.
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel represents the RSS channel.
type Channel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []RSSItem `xml:"item"`
	Generator     string    `xml:"generator"`
}

// RSSItem represents a single RSS item.
type RSSItem struct {
	Title       string     `xml:"title"`
	Description string     `xml:"description"`
	Link        string     `xml:"link"`
	GUID        string     `xml:"guid"`
	PubDate     string     `xml:"pubDate"`
	Categories  []Category `xml:"category,omitempty"`
	Enclosure   *Enclosure `xml:"enclosure"`
}

// Category maps RSS category.
type Category struct {
	Term string `xml:",chardata"`
}

// Enclosure represents the image attachment.
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length int    `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// BuildRSS constructs an RSS 2.0 feed from pins.
func BuildRSS(pins []Pin, baseURL string) (string, error) {
	items := make([]RSSItem, 0, len(pins))
	for _, p := range pins {
		cats := []Category{}
		if p.Tags != "" {
			for _, t := range splitTags(p.Tags) {
				cats = append(cats, Category{Term: t})
			}
		}

		itemLink := fmt.Sprintf("%s/image/%d", trimTrailingSlash(baseURL), p.ID)
		items = append(items, RSSItem{
			Title:       p.Title,
			Description: p.Description,
			Link:        itemLink,
			GUID:        p.GUID,
			PubDate:     p.PubDate.UTC().Format(time.RFC1123Z),
			Categories:  cats,
			Enclosure: &Enclosure{
				URL:    itemLink,
				Length: len(p.Data),
				Type:   p.MimeType,
			},
		})
	}

	rss := RSS{
		Version: "2.0",
		Channel: Channel{
			Title:         "Pinterest AI Pins",
			Link:          baseURL,
			Description:   "AI-enriched Pinterest pins feed",
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:         items,
			Generator:     "pin-uploader",
		},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(rss); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func trimTrailingSlash(in string) string {
	if len(in) > 0 && in[len(in)-1] == '/' {
		return in[:len(in)-1]
	}
	return in
}

func splitTags(tags string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(tags); i++ {
		if i == len(tags) || tags[i] == ',' {
			if i > start {
				segment := tags[start:i]
				out = append(out, trimSpace(segment))
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
