package tools

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"coreheadlines/feeds"
	"coreheadlines/typesPkg"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type AtomFeed struct {
	Entries []AtomEntry `xml:"entry"`
}

type AtomEntry struct {
	Title string `xml:"title"`
	Link  struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Content string `xml:"content"`
	ID      string `xml:"id"`
}

type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	GUID     string `xml:"guid"`
	ItemID   string `xml:"itemID"`
	Comments string `xml:"comments"`
	AtomLink struct {
		Href string `xml:"href,attr"`
	} `xml:"http://www.w3.org/2005/Atom link"`
}

type SlashdotRDF struct {
	Items []SlashdotItem `xml:"item"`
}

type SlashdotItem struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

func ParseRSSFeed(ctx context.Context, userAgents typesPkg.Agents, feed feeds.FeedConfig) ([]typesPkg.MainStruct, error) {
	client := &http.Client{
		Timeout: 40 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feed.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var selectedUserAgent string
	switch feed.Agent {
	case "chrome":
		selectedUserAgent = userAgents.Chrome
	case "reader":
		selectedUserAgent = userAgents.Reader
	case "bot":
		selectedUserAgent = userAgents.Bot
	default:
		selectedUserAgent = userAgents.Bot
	}

	req.Header.Set("User-Agent", selectedUserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")

	if feed.EnhancedHeaders {
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if feed.Source == "slashdot" {
		reader := transform.NewReader(bytes.NewReader(body), charmap.ISO8859_1.NewDecoder())
		convertedBody, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to convert encoding: %w", err)
		}

		// Remove the encoding declaration since we've converted to UTF-8
		bodyStr := string(convertedBody)
		bodyStr = strings.Replace(bodyStr, `encoding="ISO-8859-1"`, `encoding="UTF-8"`, 1)
		body = []byte(bodyStr)
	}

	if strings.HasPrefix(feed.Source, "rand-") {
		return parseRANDHTML(body, feed)
	}

	var posts []typesPkg.MainStruct

	if feed.Source == "slashdot" {
		var slashdotRDF SlashdotRDF
		if err := xml.Unmarshal(body, &slashdotRDF); err != nil {
			return nil, fmt.Errorf("failed to parse Slashdot RDF XML: %w", err)
		}

		for _, item := range slashdotRDF.Items {
			title := html.UnescapeString(strings.TrimSpace(item.Title))
			link := strings.TrimSpace(item.Link)

			if title == "" || link == "" {
				continue
			}

			post := typesPkg.MainStruct{
				GUID:   link,
				Title:  title,
				Header: feed.Header,
				Source: feed.Source,
				Link:   link,
				Topic:  feed.Topic,
			}

			posts = append(posts, post)
		}
		// Check if it's an Atom feed
	} else if strings.HasPrefix(feed.Source, "reddit-") || feed.Source == "eurostat" || strings.HasPrefix(feed.Source, "github-") {
		var atomFeed AtomFeed
		if err := xml.Unmarshal(body, &atomFeed); err != nil {
			return nil, fmt.Errorf("failed to parse Atom XML: %w", err)
		}

		for _, entry := range atomFeed.Entries {
			if strings.TrimSpace(entry.Title) == "" {
				continue
			}

			link := strings.TrimSpace(entry.Link.Href)

			post := typesPkg.MainStruct{
				GUID:   strings.TrimSpace(entry.ID),
				Title:  strings.TrimSpace(entry.Title),
				Header: feed.Header,
				Source: feed.Source,
				Link:   link,
				Topic:  feed.Topic,
			}

			posts = append(posts, post)
		}
	} else {
		// Parse as RSS
		var rss RSS
		if err := xml.Unmarshal(body, &rss); err != nil {
			return nil, fmt.Errorf("failed to parse RSS XML: %w", err)
		}

		for _, item := range rss.Channel.Items {

			title := strings.TrimSpace(item.Title)
			link := strings.TrimSpace(item.Link)

			if link == "" && item.AtomLink.Href != "" {
				link = strings.TrimSpace(item.AtomLink.Href)
			}

			if link == "" && strings.TrimSpace(item.GUID) != "" {
				link = strings.TrimSpace(item.GUID)
			}

			if title == "" || link == "" {
				continue
			}

			guid := strings.TrimSpace(item.GUID)
			if guid == "" {
				guid = strings.TrimSpace(item.ItemID)
			}

			if guid == "" {
				guid = link
			}

			post := typesPkg.MainStruct{
				GUID:   guid,
				Title:  strings.TrimSpace(item.Title),
				Header: feed.Header,
				Source: feed.Source,
				Link:   link,
				Topic:  feed.Topic,
			}

			posts = append(posts, post)
		}
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no news releases found in feed")
	}

	return posts, nil
}
