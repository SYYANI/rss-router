package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"
)

var (
	client *http.Client
	cache  struct {
		sync.RWMutex
		content []byte
		expiry  time.Time
	}
)

func init() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Transport: tr}
}

func fetchURLContent(url string) ([]byte, error) {
	cache.RLock()
	if time.Now().Before(cache.expiry) {
		content := cache.content
		cache.RUnlock()
		return content, nil
	}
	cache.RUnlock()

	log.Printf("Fetching URL: %s", url)
	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the URL: %v", err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("Fetched URL in %.2f seconds", time.Since(start).Seconds())

	cache.Lock()
	cache.content = content
	cache.expiry = time.Now().Add(5 * time.Minute)
	cache.Unlock()

	return content, nil
}

func parseArticle(article *goquery.Selection, baseURL string) *feeds.Item {
	titleTag := article.Find("h3.article-title a")
	title := titleTag.Find("span").Text()
	link, _ := titleTag.Attr("href")
	link = baseURL + link

	timeTag := article.Find("time.entry-date.published")
	publishedDate, _ := timeTag.Attr("datetime")

	contentTag := article.Find("div.article-content p")
	description := contentTag.Text()
	if description == "" {
		description = "No description available"
	}

	return &feeds.Item{
		Title:       title,
		Link:        &feeds.Link{Href: link},
		Description: description,
		Created:     parseTime(publishedDate),
	}
}

func parseTime(dateStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, dateStr)
	return t
}

func generateRSS(w http.ResponseWriter, r *http.Request) {
	log.Println("RSS generation started")
	start := time.Now()

	url := ""
	content, err := fetchURLContent(url)
	if err != nil {
		log.Printf("Error fetching URL: %v", err)
		http.Error(w, "Failed to fetch the URL", http.StatusInternalServerError)
		return
	}

	log.Println("Parsing HTML content")
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		log.Printf("Error parsing HTML: %v", err)
		http.Error(w, "Failed to parse HTML", http.StatusInternalServerError)
		return
	}

	articles := doc.Find("article")
	log.Printf("Found %d articles", articles.Length())

	log.Println("Processing articles")
	var items []*feeds.Item
	articles.Each(func(i int, s *goquery.Selection) {
		items = append(items, parseArticle(s, url))
	})

	log.Println("Generating RSS feed")
	feed := &feeds.Feed{
		Title:       "Generated RSS Feed",
		Link:        &feeds.Link{Href: url},
		Description: "RSS feed generated from the provided blog URL",
		Created:     time.Now(),
		Items:       items,
	}

	rss, err := feed.ToRss()
	if err != nil {
		log.Printf("Error generating RSS: %v", err)
		http.Error(w, "Failed to generate RSS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(rss))

	log.Printf("RSS generation completed in %.2f seconds", time.Since(start).Seconds())
}

func main() {
	http.HandleFunc("/generate_rss", generateRSS)
	log.Println("Server starting on :4000")
	log.Fatal(http.ListenAndServe(":4000", nil))
}
