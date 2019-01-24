package main

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"log"
)

type DbConfig struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

func (conf DbConfig) Connect() (*sql.DB, error) {
	config := mysql.Config{
		User:                 conf.Username,
		Passwd:               conf.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%v:%v", conf.Hostname, conf.Port),
		DBName:               conf.Database,
		AllowNativePasswords: true,
	}
	dsn := config.FormatDSN()
	log.Printf("Using %v to connect to database\n", dsn)
	return sql.Open(
		"mysql",
		dsn,
	)
}

func CloseRows(rows *sql.Rows) {
	if rows == nil {
		return
	}
	_ = rows.Close()
}
