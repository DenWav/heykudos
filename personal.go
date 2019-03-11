package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/nlopes/slack"
)

func Personal(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {

	fmt.Println("You have called the personal leaderboard.")

	var rows *sql.Rows
	var err error

	emojiTexts, emojis := EmojiMatch(ev)

	helpUser, err := GetUser(ev.User, rtm, db)

	if err != nil {
		log.Printf("Error while querying for Help User: %v\n", err)
		return
	}

	var requestId int

	err = db.QueryRow(fmt.Sprintf(`
			SELECT id
			FROM users
			WHERE username = "%v";`, helpUser.Username)).Scan(&requestId)

	fmt.Println(requestId)

	if err != nil {
		log.Printf("Error while querying for userId: %v\n", err)
		return
	}

	if len(emojis) == 0 {
		rows, err = db.Query(fmt.Sprintf(`
			SELECT t.sender, t.emoji, t.count, u.username
			FROM kudos t
				INNER JOIN users u ON t.sender = u.id
			WHERE recipient = %d
			
			GROUP BY emoji;`, requestId))
	} else {
		rows, err = db.Query(fmt.Sprintf(`
		SELECT t.sender, t.count, u.username
		FROM kudos t
			INNER JOIN users u ON t.sender = u.id
		WHERE recipient = %d AND t.emoji IN (%v)
		GROUP BY u.username
		ORDER BY t.count DESC
		LIMIT 10;`, requestId, createParams(emojiTexts)), generify(emojiTexts)...)

	}

	if err != nil {
		log.Printf("Error while querying for My Kudos Board: %v\n", err)
		return
	}

	userKudos := make([]*UserKudos, 0, 10)
	for rows.Next() {
		userKudo := UserKudos{}
		if len(emojis) == 0 {
			err = rows.Scan(&userKudo.SenderId, &userKudo.Emoji, &userKudo.Count, &userKudo.SenderName)
		} else {
			err = rows.Scan(&userKudo.SenderId, &userKudo.Count, &userKudo.SenderName)
		}

		if err != nil {
			log.Printf("Failed to get user count data: %v\n", err)
			return
		}

		userKudos = append(userKudos, &userKudo)
		fmt.Println(userKudo)
	}
	defer CloseRows(rows)

	attachments := MyBoard(emojiTexts, userKudos)
	_, _, err = rtm.PostMessage(ev.Channel, slack.MsgOptionUsername(BotUsername), slack.MsgOptionPostEphemeral(helpUser.SlackId), slack.MsgOptionAttachments(attachments...))

	if err != nil {
		log.Printf("Error while sending message to %v: %v\n", ev.Channel, err)
	}

}

type UserKudos struct {
	SenderId   int
	Emoji      string
	Count      int
	SenderName string
}

func EmojiMatch(ev *slack.MessageEvent) ([]string, [][]string) {
	// Find emojis to specify for leaderboard
	emojis := emojiPattern.FindAllStringSubmatch(ev.Text, -1)
	emojiTexts := make([]string, 0, len(emojis))
	for _, emoji := range emojis {
		emojiTexts = append(emojiTexts, emoji[1])
	}
	return emojiTexts, emojis
}

func MyBoard(emojiTexts []string, userKudo []*UserKudos) []slack.Attachment {
	sb := strings.Builder{}
	if len(emojiTexts) == 0 {
		sb.WriteString("all")
	} else {
		for i, text := range emojiTexts {
			if i != 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf(":%v:", text))
		}
	}

	attachments := []slack.Attachment{
		{
			Color:      "0C9FE8",
			MarkdownIn: []string{"text", "pretext"},
			Pretext:    fmt.Sprintf("%v %s (%v)", TeamName, "My Kudos", sb.String()),
			Text:       formatMyBoardCounts(userKudo),
		},
	}
	return attachments
}

func formatMyBoardCounts(userKudo []*UserKudos) string {
	builder := strings.Builder{}

	for i, userCount := range userKudo {
		if i != 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("%v. `%v` :%v: `%v`", i+1, userCount.SenderName, userCount.Emoji, userCount.Count))
	}

	return builder.String()
}
