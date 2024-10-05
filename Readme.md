# RSS Router

RSS Router is a flexible and powerful tool for generating and managing RSS feeds from various websites. It supports both scraping web pages to create RSS feeds and fetching existing RSS feeds, making it adaptable to a wide range of scenarios.

## Features

- Generate RSS feeds from websites that don't offer their own
- Fetch existing RSS feeds from websites that provide them
- Configurable via YAML for easy addition and management of multiple sites
- Caching mechanism to reduce load on target websites
- Support for HTML content in RSS feeds
- Flexible selectors for parsing different website structures

## Prerequisites

- Go 1.16 or higher
- Dependencies:
  - github.com/PuerkitoBio/goquery
  - github.com/gorilla/feeds
  - gopkg.in/yaml.v2

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/SYYANI/rss-router.git
   cd rss-router
   ```

2. Install dependencies:
   ```
   go mod tidy
   ```

## Configuration

Create a `config.yaml` file in the project root directory. Here's an example configuration:

```yaml
sites:
  site1:
    url: "https://example.com"
    title: "Example Blog"
    description: "RSS feed generated from Example blog"
    article_selector: "article"
    title_selector: "h2.post-title"
    link_selector: "h2.post-title a"
    date_selector: "time.published"
    content_selector: "div.post-content"
    date_format: "2006-01-02T15:04:05-07:00"
    link_attribute_name: "href"
  
  site2:
    url: "https://anotherblog.com"
    title: "Another Blog with RSS"
    description: "RSS feed from Another Blog"
    existing_rss_url: "https://anotherblog.com/feed.xml"
```

## Usage

1. Build the project:
   ```
   go build
   ```

2. Run the server:
   ```
   ./rss-router
   ```

3. Access RSS feeds:
   - For a site configured as `site1` in your YAML file: `http://localhost:4000/generate_rss?site=site1`
   - For a site configured as `site2`: `http://localhost:4000/generate_rss?site=site2`

## Adding New Sites

To add a new site, simply add a new entry to your `config.yaml` file. If the site provides its own RSS feed, use the `existing_rss_url` field. Otherwise, provide the necessary selectors for scraping the site.

## Customization

You can customize the caching duration by modifying the `fetchURLContent` function in `main.go`. The default cache expiration is set to 5 minutes.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

RVVSoC is under the [BSD-2-Clause license](https://github.com/SYYANI/rvvsoc/blob/main/LICENSE). See the [LICENSE](./LICENSE) file for details.
