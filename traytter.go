package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/caseymrm/menuet"
)

// Tweet represents one tweeter
type Tweet struct {
	Text      string
	Href      string
	Timestamp time.Time
}

var lastFetched time.Time
var lastTweets []Tweet

func recentTweets(username string) ([]Tweet, error) {
	var err error
	if lastFetched.Before(time.Now().Add(-10 * time.Minute)) {
		lastFetched = time.Now()
		url := "https://twitter.com/" + username
		log.Printf("Fetching %s", url)
		resp, geterr := http.Get(url)
		if geterr != nil {
			return lastTweets, geterr
		}
		if err != nil {
			return nil, err
		}
		lastTweets, err = parseTweets(resp.Body)
	}
	return lastTweets, nil
}

func parseTweets(r io.Reader) ([]Tweet, error) {
	tweets := make([]Tweet, 0)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	doc.Find(".tweet").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		href, exists := s.Find("a.tweet-timestamp").Attr("href")
		if !exists {
			return
		}
		timestamp, exists := s.Find("._timestamp").Attr("data-time")
		if !exists {
			log.Printf("No timestamp %s", href)
			return
		}
		unixTime, err := strconv.ParseInt(timestamp, 10, 0)
		if err != nil {
			log.Printf("Bad timestamp %s: %v", timestamp, err)
			return
		}
		parsedTime := time.Unix(unixTime, 0)
		if err != nil {
			log.Printf("Bad timestamp %s: %v", timestamp, err)
			return
		}
		tweets = append(tweets, Tweet{
			Href:      href,
			Text:      s.Find(".tweet-text").Text(),
			Timestamp: parsedTime,
		})
	})
	sort.Slice(tweets, func(i, j int) bool {
		return tweets[j].Timestamp.Before(tweets[i].Timestamp)
	})
	return tweets[0:10], nil
}

func checkTwitter() {
	ticker := time.NewTicker(10 * time.Minute)
	for ; true; <-ticker.C {
		tweets, err := recentTweets("wirecutterdeals")
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		title := "🐦"
		if len(tweets) > 0 {
			title = fmt.Sprintf("🐥 %s", tweets[0].Text[0:20])
		}
		menuet.App().SetMenuState(&menuet.MenuState{
			Title: title,
			Items: menuItems(tweets),
		})
	}
}

func menuItems(tweets []Tweet) []menuet.MenuItem {
	items := make([]menuet.MenuItem, 0, len(tweets))
	for _, tweet := range tweets {
		text := tweet.Text
		if len(text) > 41 {
			text = fmt.Sprintf("%s...", tweet.Text[0:40])
		}
		items = append(items, menuet.MenuItem{
			Text:     text,
			Callback: tweet.Href,
			Children: []menuet.MenuItem{
				menuet.MenuItem{
					Text:     tweet.Text,
					Callback: tweet.Href,
				},
			},
		})
	}
	return items
}

func handleClicks(callback chan string) {
	for clicked := range callback {
		go handleClick(clicked)
	}
}

func handleClick(clicked string) {
	exec.Command("open", "https://twitter.com"+clicked).Run()
}

func main() {
	go checkTwitter()
	app := menuet.App()
	app.Name = "Traytter"
	app.Label = "com.github.caseymrm.traytter"
	clickChannel := make(chan string)
	app.Clicked = clickChannel
	app.MenuOpened = func() []menuet.MenuItem {
		tweets, err := recentTweets("wirecutterdeals")
		if err != nil {
			log.Printf("Error: %v", err)
			return nil
		}
		return menuItems(tweets)
	}
	go handleClicks(clickChannel)
	app.RunApplication()
}
