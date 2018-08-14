package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/caseymrm/menuet"
)

var fetched time.Time
var tweetsOnce sync.Once

func fetchAllTweets() (err error) {
	tweetsOnce.Do(func() {
		menuet.Defaults().Unmarshal("usernames", &usernames)
		tweets = make(map[string][]Tweet)
	})
	if fetched.After(time.Now().Add(-9 * time.Minute)) {
		return fmt.Errorf("Called too frequently (%v > %v)", fetched, time.Now().Add(-9*time.Minute))
	}
	for ind, username := range usernames {
		newUsername, newTweets, err := fetchTweets(username)
		tweets[newUsername] = newTweets
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

func fetchTweets(username string) (string, []Tweet, error) {
	var err error
	fetched = time.Now()
	url := "https://twitter.com/" + username
	log.Printf("Fetching %s", url)
	resp, geterr := http.Get(url)
	if geterr != nil {
		return "", nil, geterr
	}
	if err != nil {
		return "", nil, err
	}
	newTweets, newUsername, err := parseTweets(username, resp.Body)
	log.Printf("Got %d tweets for %s", len(newTweets), newUsername)
	return newUsername, newTweets, err
}

func parseTweets(username string, r io.Reader) ([]Tweet, string, error) {
	tweets := make([]Tweet, 0)
	newUsername := ""
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
		author := parts[1]
		if newUsername == "" && strings.EqualFold(author, username) {
			newUsername = author
		}
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
		tweet := Tweet{
			ID:        id,
			Username:  newUsername,
			Author:    author,
			Text:      s.Find(".tweet-text").Text(),
			Timestamp: parsedTime,
		}
		avatar, exists := s.Find("img.avatar").Attr("src")
		if exists {
			tweet.AvatarURL = avatar
		}
		tweets = append(tweets, tweet)
	})
	sort.Slice(tweets, func(i, j int) bool {
		return tweets[j].Timestamp.Before(tweets[i].Timestamp)
	})
	if len(tweets) > 10 {
		tweets = tweets[0:10]
	}
	return tweets, newUsername, nil
}
