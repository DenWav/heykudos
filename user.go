package main

import (
	"database/sql"
	"fmt"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"log"
	"strings"
)

type User struct {
	Id       int64
	SlackId  string
	Username string
}

func GetUser(username string, rtm *slack.RTM, db *sql.DB) (*User, error) {
	rows, err := db.Query("SELECT id, slack_id, username FROM users WHERE slack_id = ?", username)
	if err != nil {
		return nil, err
	}
	defer CloseRows(rows)

	if rows.Next() {
		user := User{}
		err = rows.Scan(&user.Id, &user.SlackId, &user.Username)
		if err != nil {
			return nil, err
		}
		return &user, nil
	} else {
		info, err := rtm.GetUserInfo(username)
		if err != nil {
			return nil, err
		}

		user := User{0, info.ID, info.Name}
		res, err := db.Exec("INSERT INTO users (slack_id, username) VALUES (?, ?)", info.ID, info.Name)
		if err != nil {
			return nil, userInsertError(info, err)
		}

		id, err := res.LastInsertId()
		if err != nil {
			return nil, userInsertError(info, err)
		}

		user.Id = id

		return &user, nil
	}
}

func userInsertError(info *slack.User, err error) error {
	return errors.Wrap(err, fmt.Sprintf("failed to insert new user %v, slack_id %v", info.Name, info.ID))
}

func GiveKudos(from *User, to *User, db *sql.DB, rtm *slack.RTM, ev *slack.MessageEvent, left int, emojis ...string) {
	emojiCounts := make(map[string]int64)
	for _, emoji := range emojis {
		emojiCounts[emoji] += 1
	}

	successfulSends := make([]*Sent, 0, len(emojiCounts))

	for emoji, count := range emojiCounts {
		_, err := db.Exec(`
			INSERT INTO kudos (sender, recipient, emoji, count)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				count = count + ?
		`, from.Id, to.Id, emoji, count, count)

		if err != nil {
			failGivingKudos(from, to, rtm, err)
			continue
		}

		successfulSends = append(successfulSends, &Sent{emoji, count})
	}

	giveString := createGiveString(successfulSends)
	var leftString string
	if left == 0 {
		leftString = "You don't have any kudos left to give today."
	} else {
		leftString = fmt.Sprintf("You have %v kudos left to give today.", left)
	}
	SendMessage(from, fmt.Sprintf("You just sent the following kudos to `%v`: (%v). %v", to.Username, giveString, leftString), rtm)

	urlTemplate := "https://%v.slack.com/archives/%v/p%v"
	url := fmt.Sprintf(urlTemplate, DomainText, ev.Channel, strings.Replace(ev.Msg.Timestamp, ".", "", 1))
	SendMessage(to, fmt.Sprintf("You just received kudos (%v) from `%v`! (%v)", giveString, from.Username, url), rtm)
}

type Sent struct {
	Emoji string
	Count int64
}

func createGiveString(sends []*Sent) string {
	var builder = strings.Builder{}

	for i, send := range sends {
		builder.WriteString(fmt.Sprintf(":%v:: `%v`", send.Emoji, send.Count))
		if i != len(sends)-1 {
			builder.WriteString(", ")
		}
	}

	return builder.String()
}

func failGivingKudos(from *User, to *User, rtm *slack.RTM, err error) {
	log.Printf("Failed to give kudos to %v from %v: %v\n", from.Username, to.Username, err)
	SendMessage(from, fmt.Sprintf("Sorry, something went wrong while trying to give %v kudos", to.Username), rtm)
}

func SendMessage(user *User, message string, rtm *slack.RTM) {
	slackUser, err := rtm.GetUserInfo(user.SlackId)
	if err != nil {
		log.Printf("Failed to get user info for %v: %v", user.Username, err)
		return
	}
	if slackUser.IsBot {
		return
	}

	_, _, channelId, err := rtm.OpenIMChannel(user.SlackId)
	if err != nil {
		log.Printf("Failed to open channel to user %v: %v", user.Username, err)
		return
	}

	log.Printf("Attempting to send the following message to %v: %v\n", user.Username, message)
	_, _, err = rtm.PostMessage(channelId, slack.MsgOptionText(message, false))

	if err != nil {
		log.Printf("Failed to send message to user %v: %v", user.Username, err)
	}
}
