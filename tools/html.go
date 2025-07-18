package tools

import (
	"coreheadlines/feeds"
	"coreheadlines/typesPkg"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseRANDHTML(body []byte, feed feeds.FeedConfig) ([]typesPkg.MainStruct, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var posts []typesPkg.MainStruct

	doc.Find("li[data-relevancy='1.0']").Each(func(i int, s *goquery.Selection) {
		// Extract href from <a> tag
		link, exists := s.Find("a").Attr("href")
		if !exists {
			return
		}

		// Extract title from <h3 class="title">
		title := strings.TrimSpace(s.Find("h3.title").Text())
		if title == "" {
			return
		}

		// Make sure link is absolute URL
		if strings.HasPrefix(link, "/") {
			link = "https://www.rand.org" + link
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
	})

	if len(posts) == 0 {
		return nil, fmt.Errorf("no articles found in RAND HTML")
	}

	return posts, nil
}
