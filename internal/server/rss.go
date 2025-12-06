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
	Enclosure   *Enclosure `xml:"enclosure"`
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
		itemLink := fmt.Sprintf("%s/image/%d", trimTrailingSlash(baseURL), p.ID)
		items = append(items, RSSItem{
			Title:       p.Title,
			Description: p.Description,
			Link:        itemLink,
			GUID:        p.GUID,
			PubDate:     p.PubDate.UTC().Format(time.RFC1123Z),
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
