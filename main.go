package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"
	"gopkg.in/yaml.v2"
)

// SiteConfig represents the configuration for a single website
type SiteConfig struct {
	URL               string `yaml:"url"`
	Title             string `yaml:"title"`
	Description       string `yaml:"description"`
	ArticleSelector   string `yaml:"article_selector"`
	TitleSelector     string `yaml:"title_selector"`
	LinkSelector      string `yaml:"link_selector"`
	DateSelector      string `yaml:"date_selector"`
	ContentSelector   string `yaml:"content_selector"`
	DateFormat        string `yaml:"date_format"`
	LinkAttributeName string `yaml:"link_attribute_name"`
	ExistingRSSURL    string `yaml:"existing_rss_url"` // New field for existing RSS URL
}

// Config represents the overall configuration
type Config struct {
	Sites map[string]SiteConfig `yaml:"sites"`
}

var (
	client *http.Client
	cache  struct {
		sync.RWMutex
		content map[string][]byte
		expiry  map[string]time.Time
	}
	config Config
)

func init() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Transport: tr}
	cache.content = make(map[string][]byte)
	cache.expiry = make(map[string]time.Time)

	// Load configuration
	configData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
}

func fetchURLContent(url string) ([]byte, error) {
	cache.RLock()
	if time.Now().Before(cache.expiry[url]) {
		content := cache.content[url]
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
	cache.content[url] = content
	cache.expiry[url] = time.Now().Add(5 * time.Minute)
	cache.Unlock()

	return content, nil
}

func parseTime(dateStr, format string) time.Time {
	t, err := time.Parse(format, dateStr)
	if err != nil {
		log.Printf("Error parsing time: %v. Using current time instead.", err)
		return time.Now()
	}
	return t
}

func parseArticle(article *goquery.Selection, siteConfig SiteConfig) *feeds.Item {
	titleTag := article.Find(siteConfig.TitleSelector)
	title := titleTag.Text()
	
	linkTag := article.Find(siteConfig.LinkSelector)
	link, _ := linkTag.Attr(siteConfig.LinkAttributeName)
	if !strings.HasPrefix(link, "http") {
		link = siteConfig.URL + link
	}

	dateTag := article.Find(siteConfig.DateSelector)
	publishedDate, _ := dateTag.Attr("datetime")

	contentTag := article.Find(siteConfig.ContentSelector)
	
	// Convert internal links to absolute URLs
	contentTag.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.HasPrefix(href, "/") {
			s.SetAttr("href", siteConfig.URL+href)
		}
	})

	// Convert internal image sources to absolute URLs
	contentTag.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && strings.HasPrefix(src, "/") {
			s.SetAttr("src", siteConfig.URL+src)
		}
	})

	// Get the HTML content
	description, _ := contentTag.Html()
	if description == "" {
		description = "No description available"
	}

	// Wrap the HTML content with a comment indicating it's HTML
	description = fmt.Sprintf("<!-- HTML content start -->\n%s\n<!-- HTML content end -->", description)

	created := parseTime(publishedDate, siteConfig.DateFormat)

	return &feeds.Item{
		Title:       title,
		Link:        &feeds.Link{Href: link},
		Description: description,
		Created:     created,
		Id:          link, // Use the link as a unique identifier
	}
}

func generateRSS(w http.ResponseWriter, r *http.Request) {
	siteName := r.URL.Query().Get("site")
	siteConfig, ok := config.Sites[siteName]
	if !ok {
		http.Error(w, "Site not found in configuration", http.StatusNotFound)
		return
	}

	log.Printf("RSS generation started for site: %s", siteName)
	start := time.Now()

	var rss string
	var err error

	if siteConfig.ExistingRSSURL != "" {
		rss, err = fetchExistingRSS(siteConfig.ExistingRSSURL)
	} else {
		rss, err = generateRSSFromScratch(siteConfig)
	}

	if err != nil {
		log.Printf("Error generating RSS: %v", err)
		http.Error(w, "Failed to generate RSS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(rss))

	log.Printf("RSS generation completed in %.2f seconds", time.Since(start).Seconds())
}

func fetchExistingRSS(url string) (string, error) {
	content, err := fetchURLContent(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch existing RSS: %v", err)
	}
	return string(content), nil
}

func generateRSSFromScratch(siteConfig SiteConfig) (string, error) {
	content, err := fetchURLContent(siteConfig.URL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch the URL: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %v", err)
	}

	articles := doc.Find(siteConfig.ArticleSelector)
	log.Printf("Found %d articles", articles.Length())

	var items []*feeds.Item
	articles.Each(func(i int, s *goquery.Selection) {
		items = append(items, parseArticle(s, siteConfig))
	})

	feed := &feeds.Feed{
		Title:       siteConfig.Title,
		Link:        &feeds.Link{Href: siteConfig.URL},
		Description: siteConfig.Description,
		Created:     time.Now(),
		Items:       items,
	}

	rss, err := feed.ToRss()
	if err != nil {
		return "", fmt.Errorf("failed to generate RSS: %v", err)
	}

	rss = strings.Replace(rss, "<rss", "<!-- Item descriptions contain HTML content -->\n<rss", 1)

	return rss, nil
}


func main() {
	http.HandleFunc("/generate_rss", generateRSS)
	log.Println("Server starting on :4000")
	log.Fatal(http.ListenAndServe(":4000", nil))
}