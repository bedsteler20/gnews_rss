package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"github.com/mmcdole/gofeed"
)

//#section xml_builder

func Open(b *strings.Builder, name string) {
	b.WriteString("<")
	b.WriteString(name)
	b.WriteString(">")
}

func OpenWithAttrs(b *strings.Builder, name string, attrs Attrs) {
	b.WriteString("<")
	b.WriteString(name)
	for k, v := range attrs {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString("=\"")
		b.WriteString(v)
		b.WriteString("\"")
	}
	b.WriteString(">")
}

func Close(b *strings.Builder, name string) {
	b.WriteString("</")
	b.WriteString(name)
	b.WriteString(">")
}

func TagBody(b *strings.Builder, name string, fn func()) {
	Open(b, name)
	fn()
	Close(b, name)
}

func TagBodyWithAttrs(b *strings.Builder, name string, attrs Attrs, fn func()) {
	OpenWithAttrs(b, name, attrs)
	fn()
	Close(b, name)
}

type Attrs map[string]string

func Tag(b *strings.Builder, name string, value string) {
	if value == "" {
		return
	}
	Open(b, name)
	b.WriteString(value)
	Close(b, name)
}

//#endsection

//#section utils

func ResolveLink(link string) (string, error) {
	res, err := http.Get(link)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// get the herf of the first a element

	// find the first <a> tag

	re := regexp.MustCompile(`<a href="([^"]*)"`)
	matches := re.FindStringSubmatch(string(body))

	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no link found")
}

//#endsection

//#section rss_builder

func ToRss(site string, feed *gofeed.Feed, w http.ResponseWriter) string {

	b := new(strings.Builder)

	TagBodyWithAttrs(b, "rss", Attrs{"version": "2.0"}, func() {
		TagBody(b, "channel", func() {
			Tag(b, "title", feed.Title)
			Tag(b, "description", feed.Description)
			Tag(b, "link", fmt.Sprintf("https://%s", site))
			Tag(b, "feedLink", feed.FeedLink)
			Tag(b, "updated", feed.Updated)
			Tag(b, "lastBuildDate", feed.Updated)
			Tag(b, "pubDate", feed.Published)
			Tag(b, "generator", feed.Generator)
			Tag(b, "published", feed.Published)
			Tag(b, "language", feed.Language)

			var wg sync.WaitGroup
			for _, item := range feed.Items {
				wg.Add(1)
				go func(item *gofeed.Item) {
					defer wg.Done()
					body, err := ToRssItem(item)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					Tag(b, "item", body)
				}(item)
			}
			wg.Wait()
		})
	})

	return b.String()
}

func ToRssItem(item *gofeed.Item) (string, error) {
	link, err := ResolveLink(item.Link)
	if err != nil {
		return "", err
	}

	b := new(strings.Builder)
	Tag(b, "title", item.Title)
	Tag(b, "description", item.Description)
	Tag(b, "link", link)
	Tag(b, "guid", item.GUID)
	Tag(b, "pubDate", item.Published)

	return b.String(), nil
}

//#endsection

//#section routes

func siteHandler(w http.ResponseWriter, r *http.Request) {
	site := strings.TrimPrefix(r.URL.Path, "/site/")
	gnewsURL := "https://news.google.com/rss/search?q=site:" + site + "&hl=en-US&gl=US&ceid=US:en"

	if site == "" {
		http.Error(w, "site is required", http.StatusBadRequest)
		return
	}

	// parse the body
	feed, err := gofeed.NewParser().ParseURL(gnewsURL)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// convert to rss
	pfeed := ToRss(site, feed, w)

	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(pfeed))

	// write the response
}

//#endsection

//#section main

func main() {
	http.HandleFunc("/site/", siteHandler)
	http.ListenAndServe(":8080", nil)
}

//#endsection
