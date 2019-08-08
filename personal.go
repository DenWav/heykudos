package main

import (
	"database/sql"
	"fmt"
	"github.com/nlopes/slack"
	"log"
	"sort"
	"strconv"
	"strings"
)

func PersonalStats(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	emojis := EmojiMatch(ev)

	user, err := GetUser(ev.User, rtm, db)

	if err != nil {
		log.Printf("Error while querying for user: %v\n", err)
		return
	}

	rcvStats := calcStats(emojis, user, db, true)
	if rcvStats == nil {
		return
	}
	gvnStats := calcStats(emojis, user, db, false)
	if gvnStats == nil {
		return
	}

	_, _, err = rtm.PostMessage(
		ev.Channel,
		slack.MsgOptionUsername(BotUsername),
		slack.MsgOptionPostEphemeral(user.SlackId),
		slack.MsgOptionAttachments(*rcvStats, *gvnStats),
	)

	if err != nil {
		log.Printf("Error while sending message to %v: %v\n", ev.Channel, err)
	}
}

func calcStats(emojis []string, user *User, db *sql.DB, received bool) *slack.Attachment {
	var rows *sql.Rows
	var err error

	var target string
	var join string
	if received {
		join = "k.sender"
		target = "k.recipient"
	} else {
		join = "k.recipient"
		target = "k.sender"
	}

	if len(emojis) == 0 {
		rows, err = db.Query(fmt.Sprintf(`
			SELECT %s, k.emoji, k.count, u.username
			FROM kudos k
				INNER JOIN users u ON %s = u.id
			WHERE %s = ?
			ORDER BY k.count DESC, u.username DESC
			`, join, join, target), user.Id)
	} else {
		rows, err = db.Query(fmt.Sprintf(`
			SELECT %s, k.emoji, k.count, u.username
			FROM kudos k
				INNER JOIN users u ON %s = u.id
			WHERE %s = ?
				AND k.emoji IN (%s)
			ORDER BY k.count DESC, u.username DESC
		`, join, join, target, createParams(emojis)), generify(emojis, user.Id)...)

	}

	if err != nil {
		log.Printf("Error while querying for My Kudos Board: %v\n", err)
		return nil
	}

	userKudos := make(map[int]*UserKudos)
	for rows.Next() {
		kudosRow := KudosRow{}
		err = rows.Scan(&kudosRow.SenderId, &kudosRow.Emoji, &kudosRow.Count, &kudosRow.SenderName)

		if err != nil {
			log.Printf("Failed to get user count data: %v\n", err)
			return nil
		}

		kudo, ok := userKudos[kudosRow.SenderId]
		if !ok {
			userKudo := UserKudos{
				SenderId:   kudosRow.SenderId,
				Kudos:      make([]*GivenKudos, 0),
				SenderName: kudosRow.SenderName,
			}
			userKudo.Kudos = append(userKudo.Kudos, &GivenKudos{
				Emoji: kudosRow.Emoji,
				Count: kudosRow.Count,
			})
			userKudo.TotalCount = kudosRow.Count
			userKudos[kudosRow.SenderId] = &userKudo
		} else {
			kudo.Kudos = append(kudo.Kudos, &GivenKudos{
				Emoji: kudosRow.Emoji,
				Count: kudosRow.Count,
			})
			kudo.TotalCount = kudo.TotalCount + kudosRow.Count
		}
	}
	defer CloseRows(rows)

	kudosList := make([]*UserKudos, 0, len(userKudos))
	for _, v := range userKudos {
		kudosList = append(kudosList, v)
	}
	sort.Slice(kudosList, func(i, j int) bool {
		if kudosList[i].TotalCount != kudosList[j].TotalCount {
			return kudosList[i].TotalCount > kudosList[j].TotalCount
		}
		return kudosList[i].SenderName < kudosList[j].SenderName
	})

	return MyBoard(emojis, kudosList, received)
}

type GivenKudos struct {
	Emoji string
	Count int
}

type UserKudos struct {
	SenderId   int
	Kudos      []*GivenKudos
	SenderName string
	TotalCount int
}

type KudosRow struct {
	SenderId   int
	Emoji      string
	Count      int
	SenderName string
}

func EmojiMatch(ev *slack.MessageEvent) []string {
	// Find emojis to specify for leaderboard
	emojis := emojiPattern.FindAllStringSubmatch(ev.Text, -1)
	return unique(flatten(emojis, 1))
}

func MyBoard(emojiTexts []string, userKudos []*UserKudos, received bool) *slack.Attachment {
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

	var title string
	if received {
		title = "Received"
	} else {
		title = "Given"
	}
	return &slack.Attachment{
		Color:      "0C9FE8",
		MarkdownIn: []string{"text", "pretext"},
		Pretext:    fmt.Sprintf("%v My %s Kudos (%v)", TeamName, title, sb.String()),
		Text:       formatMyBoardCounts(userKudos),
	}
}

func formatMyBoardCounts(userKudos []*UserKudos) string {
	builder := strings.Builder{}

	total := 0
	for i, kudos := range userKudos {
		total += kudos.TotalCount
		if i != 0 {
			builder.WriteString("\n")
		}
		lineBuilder := strings.Builder{}
		for _, gifts := range kudos.Kudos {
			lineBuilder.WriteString(fmt.Sprintf("\n\t:%v:: `%v`", gifts.Emoji, gifts.Count))
		}
		builder.WriteString(fmt.Sprintf("%v. `%v`: `%v`%v", i+1, kudos.SenderName, kudos.TotalCount, lineBuilder.String()))
	}

	return "Total Count: `" + strconv.Itoa(total) + "`\n" + builder.String()
}
