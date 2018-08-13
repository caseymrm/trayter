package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/caseymrm/menuet"
)

// Tweet represents one tweeter
type Tweet struct {
	Text      string
	ID        string
	Username  string
	Author    string
	Timestamp time.Time
}

func (t *Tweet) href() string {
	return fmt.Sprintf("https://twitter.com/%s/status/%s", t.Username, t.ID)
}

func (t *Tweet) open() {
	exec.Command("open", t.href()).Run()
}

// Item returns a short menu item for the tweet, trucated as requested
func (t *Tweet) Item(truncate int) menuet.MenuItem {
	text := t.Text
	if len(text) > truncate-2 {
		text = fmt.Sprintf("%s...", t.Text[0:truncate-3])
	}
	item := menuet.MenuItem{
		Text:       text,
		FontWeight: menuet.WeightUltraLight,
		Clicked:    t.open,
	}
	if text != t.Text {
		item.Children = func() []menuet.MenuItem {
			return t.FullItems()
		}
	}
	return item
}

// FullItems returns several menu items for the tweet
func (t *Tweet) FullItems() []menuet.MenuItem {
	lines := wrap(t.Text, 52)
	items := make([]menuet.MenuItem, 0, len(lines)+1)
	items = append(items, menuet.MenuItem{
		Text:       fmt.Sprintf("@%s - %s", t.Username, t.Timestamp.Format("Mon Jan 2 3:04pm")),
		Clicked:    t.open,
		FontWeight: menuet.WeightBold,
	})
	for _, line := range lines {
		items = append(items, menuet.MenuItem{
			Text:       line,
			Clicked:    t.open,
			FontWeight: menuet.WeightUltraLight,
		})
	}
	return items
}

func wrap(text string, width int) []string {
	lines := make([]string, 0, len(text)/width)
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}
	current := words[0]
	remaining := width - len(current)
	for _, word := range words[1:] {
		if len(word)+1 > remaining {
			lines = append(lines, current)
			current = word
			remaining = width - len(word)
		} else {
			current += " " + word
			remaining -= 1 + len(word)
		}
	}
	lines = append(lines, current)
	return lines
}

var usernames []string
var tweets map[string][]Tweet

func checkTwitter() {
	ticker := time.NewTicker(10 * time.Minute)
	for ; true; <-ticker.C {
		err := fetchAllTweets()
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		setTitle()
		menuet.App().MenuChanged()
	}
}

func setTitle() {
	title := "🐦"
	if len(usernames) > 0 && len(tweets[usernames[0]]) > 0 {
		title += getKeyword(tweets[usernames[0]][0].Text)
	}
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: title,
	})

}

func follow(username string) {
	username = strings.Trim(username, "@ ")
	for _, name := range usernames {
		if strings.EqualFold(name, username) {
			return
		}
	}
	newUsername, newTweets, err := fetchTweets(username)
	if err != nil {
		menuet.App().Alert(menuet.Alert{
			MessageText:     fmt.Sprintf("Could not fetch %s", username),
			InformativeText: err.Error(),
		})
		return
	}
	tweets[newUsername] = newTweets
	setTitle()
	usernames = append(usernames, newUsername)
	menuet.Defaults().Marshal("usernames", usernames)
}

func remove(username string) {
	for ind, name := range usernames {
		if name == username {
			usernames = append(usernames[:ind], usernames[ind+1:]...)
			delete(tweets, username)
			setTitle()
			menuet.Defaults().Marshal("usernames", usernames)
			return
		}
	}
}

func menuItems() []menuet.MenuItem {
	items := make([]menuet.MenuItem, 0, 2*len(usernames)+2)
	for _, username := range usernames {
		username := username
		if len(tweets[username]) > 0 {
			tweet := tweets[username][0]
			items = append(items, menuet.MenuItem{
				Text:    fmt.Sprintf("@%s %s", username, getKeyword(tweet.Text)),
				Clicked: tweet.open,
				Children: func() []menuet.MenuItem {
					return usernameItems(username)
				},
			})
		} else {
			items = append(items, menuet.MenuItem{
				Text: fmt.Sprintf("@%s", username),
				Children: func() []menuet.MenuItem {
					return usernameItems(username)
				},
			})
		}
	}
	items = append(items, menuet.MenuItem{
		Type: menuet.Separator,
	})
	items = append(items, menuet.MenuItem{
		Text: "Follow a user",
		Clicked: func() {
			response := menuet.App().Alert(menuet.Alert{
				MessageText: "What Twitter user would you like to follow?",
				Inputs:      []string{"@username"},
				Buttons:     []string{"Follow", "Cancel"},
			})
			if response.Button == 0 && len(response.Inputs) == 1 && response.Inputs[0] != "" {
				follow(response.Inputs[0])
			}
		},
	})
	return items
}

func usernameItems(username string) []menuet.MenuItem {
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
			tweet := tweet
			items = append(items, tweet.Item(50))
		}
	}
	items = append(items, menuet.MenuItem{
		Type: menuet.Separator,
	})
	items = append(items, menuet.MenuItem{
		Text: fmt.Sprintf("Remove @%s", username),
		Clicked: func() {
			remove(username)
		},
	})
	return items
}

func main() {
	go checkTwitter()
	app := menuet.App()
	app.Name = "Traytter"
	app.Label = "com.github.caseymrm.traytter"
	app.Children = menuItems
	app.RunApplication()
}
