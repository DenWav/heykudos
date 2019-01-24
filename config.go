package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

var BotConfig *Config

type Config struct {
	BotToken     string `json:"botToken"`
	UserToken    string `json:"userToken"`
	DbConfig     `json:"db"`
	AmountPerDay int `json:"amountPerDay"`
}

func ReadConfig() {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Failed to read configuration file: %v\n", err)
	}

	BotConfig = &Config{}
	err = json.Unmarshal(data, BotConfig)
	if err != nil {
		log.Fatalf("Failed to parse configuration file: %v\n", err)
	}
}
