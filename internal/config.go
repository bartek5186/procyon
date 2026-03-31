package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Config struct {
	AppName    string   `json:"app_name"`
	AuthDomain string   `json:"auth_domain"`
	Languages  []string `json:"languages"`
	Domains    []string `json:"domains"`
	Server     struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Database struct {
		Host     string `json:"host"`
		User     string `json:"user"`
		Password string `json:"password"`
		DbName   string `json:"dbname"`
		Port     int    `json:"port"`
	} `json:"database"`
}

func LoadConfiguration(file string) Config {
	var config Config

	configFile, err := os.Open(file)
	if err != nil {
		logrus.WithError(err).Fatal("problem loading config file")
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		logrus.WithError(err).Fatal("problem parsing config file")
	}

	return config
}

func (c Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func NewDatabaseConnection(cfg Config) *gorm.DB {
	loc := url.QueryEscape("UTC")
	tz := url.QueryEscape("'+00:00'")

	connStr := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=%s&time_zone=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DbName,
		loc,
		tz,
	)

	db, err := gorm.Open(mysql.Open(connStr), &gorm.Config{})
	if err != nil {
		logrus.WithError(err).Fatal("unable to connect to database")
	}

	return db
}
