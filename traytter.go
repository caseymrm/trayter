package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/caseymrm/menuet"
)

// Tweet represents one tweeter
type Tweet struct {
	Text      string
	ID        string
	Username  string
	Timestamp time.Time
}

// Href returns the URL for this tweet
func (t *Tweet) Href() string {
	return fmt.Sprintf("https://twitter.com/%s/status/%s", t.Username, t.ID)
}

// Item returns a menu item for the tweet, trucated as required
func (t *Tweet) Item(truncate int) menuet.MenuItem {
	text := t.Text
	if truncate > 0 && len(text) > truncate-2 {
		text = fmt.Sprintf("%s...", t.Text[0:truncate-3])
	}
	return menuet.MenuItem{
		Text:     text,
		Key:      fmt.Sprintf("tweet:%s %s", t.Username, t.ID),
		Children: text != t.Text,
	}
}

var fetched time.Time
var usernames []string
var tweets map[string][]Tweet
var tweetsOnce sync.Once

func fetchAllTweets() (err error) {
	tweetsOnce.Do(func() {
		usernames = []string{"wirecutterdeals"}
		tweets = make(map[string][]Tweet)
	})
	if fetched.After(time.Now().Add(-9 * time.Minute)) {
		return fmt.Errorf("Called too frequently (%v > %v)", fetched, time.Now().Add(-9*time.Minute))
	}
	for ind, username := range usernames {
		newUsername, err := fetchTweets(username)
		if err != nil {
			log.Printf("Error fetching %s: %v", username, err)
			continue
		}
		usernames[ind] = newUsername
	}
	sort.Slice(usernames, func(i, j int) bool {
		if len(tweets[usernames[j]]) == 0 {
			if len(tweets[usernames[i]]) == 0 {
				return usernames[j] < usernames[i]
			}
			// i exists, j does not, put i first
			return true
		} else if len(tweets[usernames[i]]) == 0 {
			// j exists, i does not, put j first
			return false
		}
		return tweets[usernames[j]][0].Timestamp.Before(tweets[usernames[i]][0].Timestamp)
	})
	return err
}

func fetchTweets(username string) (string, error) {
	var err error
	fetched = time.Now()
	url := "https://twitter.com/" + username
	log.Printf("Fetching %s", url)
	resp, geterr := http.Get(url)
	if geterr != nil {
		return "", geterr
	}
	if err != nil {
		return "", err
	}
	newTweets, newUsername, err := parseTweets(resp.Body)
	log.Printf("Got %d tweets for %s", len(newTweets), newUsername)
	tweets[newUsername] = newTweets
	return newUsername, err
}

func parseTweets(r io.Reader) ([]Tweet, string, error) {
	tweets := make([]Tweet, 0)
	username := ""
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, "", err
	}
	doc.Find(".tweet").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Find("a.tweet-timestamp").Attr("href")
		if !exists {
			return
		}
		parts := strings.Split(href, "/")
		if len(parts) != 4 {
			return
		}
		username = parts[1]
		id := parts[3]
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
			ID:        id,
			Username:  username,
			Text:      s.Find(".tweet-text").Text(),
			Timestamp: parsedTime,
		})
	})
	sort.Slice(tweets, func(i, j int) bool {
		return tweets[j].Timestamp.Before(tweets[i].Timestamp)
	})
	if len(tweets) > 10 {
		tweets = tweets[0:10]
	}
	return tweets, username, nil
}

func checkTwitter() {
	ticker := time.NewTicker(10 * time.Minute)
	for ; true; <-ticker.C {
		err := fetchAllTweets()
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		title := "🐦"
		if len(usernames) > 0 && len(tweets[usernames[0]]) > 0 {
			title = fmt.Sprintf("🐥%s", tweets[usernames[0]][0].Text[0:20])

		}
		menuet.App().SetMenuState(&menuet.MenuState{
			Title: title,
		})
	}
}

func menuItems(key string) []menuet.MenuItem {
	if key == "" {
		items := make([]menuet.MenuItem, 0, 2*len(usernames))
		for _, username := range usernames {
			if len(tweets[username]) > 0 {
				tweet := tweets[username][0]
				items = append(items, tweet.Item(30))
			}
			items = append(items, menuet.MenuItem{
				Text:     fmt.Sprintf("@%s", username),
				Key:      fmt.Sprintf("username:%s", username),
				Children: true,
			})
		}
		return items
	}
	if strings.HasPrefix(key, "username:") {
		var username string
		fmt.Sscanf(key, "username:%s", &username)
		recent := tweets[username]
		items := make([]menuet.MenuItem, 0, len(recent))
		if len(recent) == 0 {
			items = append(items, menuet.MenuItem{
				Text: "No tweets!",
			})
		} else {
			items = append(items, menuet.MenuItem{
				Text:     "Recent tweets",
				FontSize: 9,
			})
			for _, tweet := range recent {
				items = append(items, tweet.Item(30))
			}
		}
		items = append(items, menuet.MenuItem{
			Type: menuet.Separator,
		})
		items = append(items, menuet.MenuItem{
			Text: fmt.Sprintf("Remove @%s", username),
			Key:  fmt.Sprintf("remove:%s", username),
		})
		return items
	}
	if strings.HasPrefix(key, "tweet:") {
		var id string
		var username string
		fmt.Sscanf(key, "tweet:%s %s", &username, &id)
		for _, tweet := range tweets[username] {
			if tweet.ID == id {
				return []menuet.MenuItem{
					tweet.Item(0),
				}
			}
		}
		return []menuet.MenuItem{
			{
				Text: "Can't find tweet!",
			},
		}

	}

	return nil
}

func handleClick(clicked string) {
	if strings.HasPrefix(clicked, "tweet:") {
		var href string
		var username string
		fmt.Sscanf(clicked, "tweet:%s %s", &username, &href)
		exec.Command("open", "https://twitter.com"+href).Run()
		return
	}
	if strings.HasPrefix(clicked, "remove:") {
		var username string
		fmt.Sscanf(clicked, "remove:%s %s", &username)
		log.Printf("Remove %s", username)
	}
}

func main() {
	go checkTwitter()
	app := menuet.App()
	app.Name = "Traytter"
	app.Label = "com.github.caseymrm.traytter"
	app.Clicked = handleClick
	app.MenuOpened = menuItems
	app.RunApplication()
}
