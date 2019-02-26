package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
)

var (
	pingPattern  = regexp.MustCompile("<@([a-zA-Z0-9]+)>")
	emojiPattern = regexp.MustCompile("`[^`]*`|:([a-z0-9_\\-+']+):")
)

var enabledChannels = make(map[string]bool)

var (
	BotId           string
	EnableText      string
	DisableText     string
	LeaderboardText string
	TeamName        string
	DomainText      string
	BotUsername     string
	HelpText        string
)

func Init(info *slack.Info) {
	BotId = info.User.ID
	EnableText = fmt.Sprintf("<@%v> enable", BotId)
	DisableText = fmt.Sprintf("<@%v> disable", BotId)
	LeaderboardText = fmt.Sprintf("<@%v> leaderboard", BotId)
	TeamName = info.Team.Name
	DomainText = info.Team.Domain
	BotUsername = info.User.Name
	HelpText = fmt.Sprintf("<@%v> help", BotId)
}

func MessageHandler(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	if ev.Text == EnableText {
		EnableChannel(ev, rtm, db)
		return
	}

	if !checkChannelEnabled(ev.Channel, db) {
		return
	}

	switch {
	case ev.Text == DisableText:
		DisableChannel(ev, rtm, db)
	case strings.HasPrefix(ev.Text, LeaderboardText):
		leaderboard(ev, rtm, db)
	case strings.HasPrefix(ev.Text, HelpText):
		HelpMessage(ev, rtm, db)
	default:
		giveKudos(ev, rtm, db)
	}
}

func EnableChannel(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	conversation, err := rtm.GetConversationInfo(ev.Channel, true)
	if err != nil {
		log.Printf("Failed to get channel info for %v\n: %v", ev.Channel, err)
		return
	}

	if conversation.IsIM || conversation.IsMpIM {
		log.Printf("Not enabling %v, not a normal channel\n", ev.Channel)
		user, err := GetUser(ev.User, rtm, db)
		if err != nil {
			return // give up
		}
		SendMessage(user, "Sorry, you're only allowed to enable normal channels", rtm)
		return
	}

	log.Printf("Enabling channel %v\n", ev.Channel)
	enabledChannels[ev.Channel] = true
	rows, err := db.Query(`
		INSERT INTO enabled_channels (name, enabled)
		VALUES (?, TRUE)
		ON DUPLICATE KEY UPDATE
			enabled = TRUE
	`, ev.Channel)

	if err != nil {
		log.Printf("Failed to enable channel %v: %v\n", ev.Channel, err)
		return
	}
	CloseRows(rows)

	user, err := GetUser(ev.User, rtm, db)
	if err != nil {
		return
	}

	if conversation.IsPrivate {
		SendMessage(user, fmt.Sprintf("Enabled private channel #%v", conversation.Name), rtm)
	} else {
		SendMessage(user, fmt.Sprintf("Enabled channel <#%v>", ev.Channel), rtm)
	}
}

func DisableChannel(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	log.Printf("Disabling channel %v\n", ev.Channel)
	enabledChannels[ev.Channel] = false
	rows, err := db.Query("UPDATE enabled_channels SET enabled = FALSE WHERE name = ?", ev.Channel)
	if err != nil {
		log.Printf("Failed to disable channel %v: %v\n", ev.Channel, err)
		return
	}
	CloseRows(rows)

	user, err := GetUser(ev.User, rtm, db)
	if err != nil {
		return
	}

	conversation, err := rtm.GetConversationInfo(ev.Channel, true)
	if err != nil {
		return
	}

	if conversation.IsPrivate {
		SendMessage(user, fmt.Sprintf("Disabled private channel #%v", conversation.Name), rtm)
	} else {
		SendMessage(user, fmt.Sprintf("Disabled channel <#%v>", ev.Channel), rtm)
	}
}

type UserCount struct {
	Username string
	Count    int
}

func leaderboard(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	// Find emojis to specify for leaderboard
	emojis := emojiPattern.FindAllStringSubmatch(ev.Text, -1)
	emojiTexts := make([]string, 0, len(emojis))
	for _, emoji := range emojis {
		emojiTexts = append(emojiTexts, emoji[1])
	}

	var rows *sql.Rows
	var err error

	if len(emojis) == 0 {
		// sum all emojis when not specified
		rows, err = db.Query(`
			SELECT u.username, SUM(t.count)
			FROM kudos t
				INNER JOIN users u ON t.recipient = u.id
			GROUP BY u.username
			ORDER BY SUM(t.count) DESC
			LIMIT 10`)
	} else {
		rows, err = db.Query(fmt.Sprintf(`
			SELECT u.username, SUM(t.count)
			FROM kudos t
				INNER JOIN users u ON t.recipient = u.id
			WHERE t.emoji IN (%v)
			GROUP BY u.username
			ORDER BY SUM(t.count) DESC
			LIMIT 10
		`, createParams(emojiTexts)), generify(emojiTexts)...)
	}

	if err != nil {
		log.Printf("Error while querying for leaderboard: %v\n", err)
		return
	}

	defer CloseRows(rows)

	userCounts := make([]*UserCount, 0, 10)
	for rows.Next() {
		userCount := UserCount{}
		err = rows.Scan(&userCount.Username, &userCount.Count)
		if err != nil {
			log.Printf("Failed to get user count data: %v\n", err)
			return
		}

		userCounts = append(userCounts, &userCount)
	}

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
			Pretext:    fmt.Sprintf("%v Leaderboard (%v)", TeamName, sb.String()),
			Text:       formatLeaderboardCounts(userCounts),
		},
	}
	_, _, err = rtm.PostMessage(ev.Channel, slack.MsgOptionUsername(BotUsername), slack.MsgOptionAttachments(attachments...))

	if err != nil {
		log.Printf("Error while sending message to %v: %v\n", ev.Channel, err)
	}
}

// formatLeaderboardCounts takes an ordered slice of UserCounts and returns the string to be sent as a message to Slack
func formatLeaderboardCounts(userCounts []*UserCount) string {
	builder := strings.Builder{}

	for i, userCount := range userCounts {
		if i != 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("%v. `%v` `%v`", i+1, userCount.Username, userCount.Count))
	}

	return builder.String()
}

// giveKudos first checks if the message should give kudos. Messages without a pinged user (recipient) and messages
// without any emojis are not kudos messages. This function assumes the channel has already been validated as enabled.
func giveKudos(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	emojis := emojiPattern.FindAllStringSubmatch(ev.Text, -1)
	if len(emojis) == 0 {
		return
	}

	// Throw out emojis which aren't actually emojis
	validEmojis := make([]string, 0, len(emojis))
	for _, emoji := range emojis {
		if len(emoji) != 2 {
			continue
		}
		if isEmoji(emoji[1]) {
			validEmojis = append(validEmojis, emoji[1])
		}
	}

	if len(validEmojis) == 0 {
		return
	}

	// Find all pinged names
	names := flatten(pingPattern.FindAllStringSubmatch(ev.Text, -1), 1)
	if names == nil {
		return
	}

	// Prevent duplicate pings from spamming
	names = unique(names)

	// Find sender, should always succeed
	from, err := GetUser(ev.User, rtm, db)
	if err != nil {
		log.Printf("Failed to get info for user %v: %v\n", ev.Username, err)
		return
	}

	// Convert pings to actual users
	// Not all have to work (_technically_ not necessary, but matching that format is unlikely on accident)
	toSlice := make([]*User, 0, len(names))
	for _, name := range names {
		to, err := GetUser(name, rtm, db)
		if err != nil {
			// This doesn't really have to be a username
			continue
		}
		if to.Id == from.Id {
			SendMessage(from, "Sorry, but you can't give yourself kudos!", rtm)
			return
		}
		toSlice = append(toSlice, to)
	}

	// If there aren't any actual pings, quit
	if len(toSlice) == 0 {
		return
	}

	if len(toSlice) > 1 && len(validEmojis) > 1 && len(toSlice) != len(validEmojis) {
		SendMessage(from, fmt.Sprintf("Sorry, but I couldn't figure out how to give your kudos. You listed "+
			"more than one recipient and more than one emoji, but the number of each doesn't match! I saw `%v` "+
			"recipients and `%v` emojis.", len(toSlice), len(validEmojis)), rtm)
		SendMessage(from, "You can list only one emoji which will go to everyone, or multiple emojis to go "+
			"to one person. But multiple emojis to multiple people have to match counts!", rtm)
		return
	}

	left := checkRateLimit(from, toSlice, validEmojis, db, rtm)
	if left < 0 {
		return
	}

	if len(toSlice) > 1 {
		// Multiple names, match emojis to names (if multiple emojis are listed)
		for i, to := range toSlice {
			if len(validEmojis) > 1 {
				GiveKudos(from, to, db, rtm, ev, left, validEmojis[i])
			} else {
				GiveKudos(from, to, db, rtm, ev, left, validEmojis[0])
			}
		}
	} else {
		// Single name, give all emojis listed
		GiveKudos(from, toSlice[0], db, rtm, ev, left, validEmojis...)
	}
}

// checkChannelEnabled determines if a particular channel is enabled (turned on with @heykudos enable). The state is
// stored in the database, but an in-memory cache `enabledChannels` is used after initial reads.
func checkChannelEnabled(channelName string, db *sql.DB) bool {
	val, ok := enabledChannels[channelName]
	if ok {
		return val
	}

	rows, err := db.Query("SELECT enabled FROM enabled_channels WHERE name = ?", channelName)
	if err != nil {
		log.Printf("Error while querying enabled_channels: %v\n", err)
		return false
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var enabled bool
	if rows.Next() {
		err = rows.Scan(&enabled)
		if err != nil {
			log.Printf("Error while scanning query result: %v\n", err)
			return false
		}
	}

	enabledChannels[channelName] = enabled
	return enabled
}

func checkRateLimit(from *User, toSlice []*User, validEmojis []string, db *sql.DB, rtm *slack.RTM) int {
	// Figure out how many they want to give vs how many they can give at this point
	var give int
	if len(toSlice) > 1 {
		give = len(toSlice)
	} else {
		give = len(validEmojis)
	}

	// Remove old entries to reset for the day
	rows, err := db.Query(`DELETE FROM rate WHERE time < CURRENT_DATE()`)
	if err != nil {
		log.Printf("Failed to remove old kudos: %v\n", err)
		return -1
	}
	CloseRows(rows)

	// Count total for today
	rows, err = db.Query(`
		SELECT coalesce(r.count, 0)
		FROM rate r
		WHERE r.user_id = ?
`, from.Id)
	if err != nil {
		log.Printf("Failed to query for rate limits: %v\n", err)
		return -1
	}
	defer CloseRows(rows)

	var count int
	if rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			log.Printf("Failed to get rate limit count: %v\n", err)
			return -1
		}
	} else {
		count = 0
	}

	switch {
	case count >= BotConfig.AmountPerDay:
		log.Printf("%v rate limited\n", from.Username)
		message := fmt.Sprintf("Sorry, you're out of kudos to give for now. You can only give %v every 24 hours.", BotConfig.AmountPerDay)
		SendMessage(from, message, rtm)
		return -1
	case (count + give) > BotConfig.AmountPerDay:
		log.Printf("%v rate limited\n", from.Username)
		message := fmt.Sprintf("Sorry, you tried to give %v kudos, but you only have %v kudos left to give today.", give, BotConfig.AmountPerDay-count)
		SendMessage(from, message, rtm)
		return -1
	}

	rows, err = db.Query(`
		INSERT INTO rate (user_id, count) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			count = count + ?
	`, from.Id, give, give)
	if err != nil {
		log.Printf("Failed to insert into rate limit table: %v\n", err)
		return -1
	}
	CloseRows(rows)

	return BotConfig.AmountPerDay - (count + give)
}

//helpMessage added 2-21-19
func HelpMessage(ev *slack.MessageEvent, rtm *slack.RTM, db *sql.DB) {
	//Get user that requested help
	helpUser, err := GetUser(ev.User, rtm, db)
	if err != nil {
		log.Printf("Failed to get info for user %v: %v\n", ev.Username, err)
		return
	}

	//TODO replace the help message string with an attachment markup
	helpString := "heykudos is a bot used to recognize someone for being awesome!\n" +

		"If you want to send someone a kudos simply @ them and send them an emoji. Any emoji will work!\n" +

		">`@username` :rainbow:\n" +

		"You can send a message along too if you like:\n" +
		">`@username` :rainbow: for being the best bot on slack!\n" +

		"You can send kudos in multiple ways:\n" +
		">Multiple kudos to one person `@username` :rainbow: :heart:\n" +
		">One kudos to multiple people `@username` `@another.username` :rainbow:\n" +
		">Multiple kudos to multiple people `@username` `@another.username` :rainbow: :heart:\n" +

		"You can show the overall leaderboard:\n" +
		">`@heykudos` leaderboard\n" +

		"Or a leaderboard for a particular emoji\n" +
		">`@heykudos` leaderboard :rainbow:\n" +

		"You are limited to 5 kudos per day to send, but can receive an unlimited amount of kudos!"
	//Post an ephemeral message to same channel the help request was made from
	_, _, err = rtm.PostMessage(ev.Channel, slack.MsgOptionUsername(BotUsername), slack.MsgOptionPostEphemeral(helpUser.SlackId), slack.MsgOptionText(helpString, false))

	//if an error occurs log it
	if err != nil {
		log.Printf("Error while sending message to %v: %v\n", ev.Channel, err)
	}

}
