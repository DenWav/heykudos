package main

import (
	"github.com/nlopes/slack"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ReadConfig()

	db, err := BotConfig.DbConfig.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\n", err)
	}
	defer func() {
		log.Printf("Shutting down\n")
		err := db.Close()
		if err != nil {
			log.Printf("Failed to close database connection properly\n")
		}
	}()

	cancel := make(chan bool)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		cancel <- true
	}()

	// The emoji API endpoint can't be accessed by the bot token (for some reason)
	// So we have this other token (which can't be used for the bot APIs) for everything else
	emojiApiKey = BotConfig.UserToken
	api := slack.New(BotConfig.BotToken)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	// Handle a few events
	// Note messages are handled async
loop:
	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.ConnectedEvent:
				log.Printf("Connected to Slack API server\n")
				Init(ev.Info)
			case *slack.MessageEvent:
				if ev.Hidden {
					continue
				}
				go MessageHandler(ev, rtm, db)
			case *slack.LatencyReport:
				log.Printf("Current latency: %v\n", ev.Value)
			case *slack.RTMError:
				log.Printf("Error: %s\n", ev.Error())
			case *slack.InvalidAuthEvent:
				log.Println("Invalid credentials")
				return
			}
		case <-cancel:
			break loop
		}
	}
}
