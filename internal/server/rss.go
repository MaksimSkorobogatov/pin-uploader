package server

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
)

// RSS represents the RSS feed root.
type RSS struct {
	XMLName    xml.Name `xml:"rss"`
	Version    string   `xml:"version,attr"`
	XMLNSMedia string   `xml:"xmlns:media,attr"`
	Channel    Channel  `xml:"channel"`
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
	Title       string       `xml:"title"`
	Description CDATA        `xml:"description"`
	Link        string       `xml:"link"`
	GUID        string       `xml:"guid"`
	PubDate     string       `xml:"pubDate"`
	Media       MediaContent `xml:"media:content"`
}

// MediaContent describes the Media RSS content block.
type MediaContent struct {
	URL    string      `xml:"url,attr"`
	Type   string      `xml:"type,attr"`
	Medium string      `xml:"medium,attr"`
	Title  *MediaTitle `xml:"media:title,omitempty"`
}

// MediaTitle holds the optional title inside media:content.
type MediaTitle struct {
	Type string `xml:"type,attr,omitempty"`
	Text string `xml:",chardata"`
}

// CDATA marshals string data as a CDATA section.
type CDATA string

// MarshalXML ensures the string value is wrapped in <![CDATA[...]]>.
func (c CDATA) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wrapper := struct {
		Inner string `xml:",innerxml"`
	}{
		Inner: "<![CDATA[" + string(c) + "]]>",
	}
	return e.EncodeElement(wrapper, start)
}

// BuildRSS constructs an RSS 2.0 feed from pins.
func BuildRSS(pins []Pin, baseURL string) (string, error) {
	items := make([]RSSItem, 0, len(pins))
	base := trimTrailingSlash(baseURL)
	for _, p := range pins {
		imageURL := fmt.Sprintf("%s/rss/image/%d", base, p.ID)
		items = append(items, RSSItem{
			Title:       p.Title,
			Description: CDATA(p.Description),
			Link:        imageURL,
			GUID:        p.GUID,
			PubDate:     p.PubDate.UTC().Format(time.RFC1123Z),
			Media: MediaContent{
				URL:    imageURL,
				Type:   p.MimeType,
				Medium: "image",
				Title: &MediaTitle{
					Type: "plain",
					Text: p.Title,
				},
			},
		})
	}

	rss := RSS{
		Version:    "2.0",
		XMLNSMedia: "http://search.yahoo.com/mrss/",
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
