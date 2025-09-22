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

	"golang.org/x/net/html/charset"
)

type AtomFeed struct {
	Entries []AtomEntry `xml:"entry"`
}

type AtomEntry struct {
	Title string `xml:"title"`
	Link  struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	ID string `xml:"id"`
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

func decodeXML(body []byte, v any) error {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.CharsetReader = charset.NewReaderLabel
	return decoder.Decode(v)
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

	var resp *http.Response
	const maxRetries = 3

	for i := range maxRetries {
		resp, err = client.Do(req)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			wait := time.Duration(500*(1<<i)) * time.Millisecond // Exponential backoff
			fmt.Printf("Attempt %d: failed to fetch %s: %v. Retrying in %v...\n", i+1, feed.URL, err, wait)
			time.Sleep(wait)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to make request after retries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var posts []typesPkg.MainStruct

	if feed.Header == "Slashdot" {
		var slashdotRDF SlashdotRDF
		if err := decodeXML(body, &slashdotRDF); err != nil {
			return nil, fmt.Errorf("failed to parse Slashdot RDF XML: %w", err)
		}

		for _, item := range slashdotRDF.Items {
			title := html.UnescapeString(strings.TrimSpace(item.Title))

			if title == "" || item.Link == "" {
				continue
			}

			post := typesPkg.MainStruct{
				GUID:   item.Link,
				Title:  title,
				Header: feed.Header,
				Link:   item.Link,
			}

			posts = append(posts, post)
		}
		// Check if it's an Atom feed
	} else if strings.HasPrefix(feed.Header, "r/") {
		var atomFeed AtomFeed
		if err := decodeXML(body, &atomFeed); err != nil {
			return nil, fmt.Errorf("failed to parse Atom XML: %w", err)
		}

		h := strings.ReplaceAll(feed.Header, " ", "")

		for _, entry := range atomFeed.Entries {
			title := strings.TrimSpace(entry.Title)
			if title == "" {
				continue
			}

			link := strings.TrimSpace(entry.Link.Href)
			candidate := strings.TrimSpace(entry.ID)

			var guid string
			if candidate != "" {
				if h != "" {
					guid = h + ":" + candidate
				} else {
					guid = candidate
				}
			} else if link != "" {
				guid = link // fallback — do NOT prefix
			} else {
				continue
			}

			post := typesPkg.MainStruct{
				GUID:   guid,
				Title:  title,
				Header: feed.Header,
				Link:   link,
			}
			posts = append(posts, post)
		}
	} else {
		// Parse as RSS
		var rss RSS
		if err := decodeXML(body, &rss); err != nil {
			return nil, fmt.Errorf("failed to parse RSS XML: %w", err)
		}

		h := strings.ReplaceAll(feed.Header, " ", "")

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

			candidate := strings.TrimSpace(item.GUID)
			if candidate == "" {
				candidate = strings.TrimSpace(item.ItemID)
			}

			var guid string
			if candidate != "" {
				if h != "" {
					guid = h + ":" + candidate
				} else {
					guid = candidate
				}
			} else {
				guid = link // fallback — do NOT prefix
			}

			post := typesPkg.MainStruct{
				GUID:   guid,
				Title:  title,
				Header: feed.Header,
				Link:   link,
			}
			posts = append(posts, post)
		}
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no news releases found in feed")
	}

	return posts, nil
}
