package main

import (
	"encoding/json"
	"github.com/nlopes/slack"
	"log"
	"net/http"
	"sync"
)

var (
	emojiApiKey string
	emojiCache  = make(map[string]bool)
	emojiMutex  = &sync.Mutex{}
)

// isEmoji checks if the given name is recognized as an emoji - either standard or custom. It keeps an in-memory cache
// of the emoji lists to keep the network calls less frequent (the calls can be quite time consuming) but any name given
// that isn't in the cache will cause a full refresh of the emoji list to account for newly added emojis.
func isEmoji(name string) (valid bool) {
	_, valid = emojiCache[name]
	if valid {
		return
	}

	pullAllEmojis()
	_, valid = emojiCache[name]
	return
}

// pullAllEmojis wraps pullStandardEmojis and pullCustomEmojis around a mutex to prevent the lists from being updated
// concurrently
func pullAllEmojis() {
	emojiMutex.Lock()
	defer emojiMutex.Unlock()

	pullStandardEmojis()
	pullCustomEmojis()
}

// pullStandardEmojis grabs the standard emoji data set for emojis that are included in the base slack package - these
// aren't necessarily slack specific, but they are what slack uses as it's default emoji set.
func pullStandardEmojis() {
	var url = "https://raw.githubusercontent.com/iamcal/emoji-data/master/emoji.json"

	type EmojiData struct {
		ShortNames []string `json:"short_names"`
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to get standard Slack emoji set: %v\n", err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to get standard Slack emoji set: %v\n", err)
		return
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("Failed to properly close standard Slack emoji request body: %v\n", err)
		}
	}()

	var data []EmojiData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("Failed to decode standard Slack emoji request: %v\n", err)
	}

	for _, emoji := range data {
		for _, name := range emoji.ShortNames {
			emojiCache[name] = true
		}
	}
}

// pullCustomEmojis queries the slack API to pull in the names of all custom emojis for the workspace. This uses the
// emojiApiKey that is separate from the bot token that is used for all other API calls. This API in particular is
// different and does not work with the bot API token.
func pullCustomEmojis() {
	api := slack.New(emojiApiKey)
	ec, err := api.GetEmoji()
	if err != nil {
		log.Printf("Failed to pull emoji list: %v", err)
		return
	}
	for k := range ec {
		emojiCache[k] = true
	}
}
