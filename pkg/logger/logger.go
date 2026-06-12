package logger

import (
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()
var log *logrus.Entry

func GetLogger() (*logrus.Entry, error) {
	logger.SetFormatter((&logrus.TextFormatter{}))
	SetGameId("none")
	return log, nil
}

func SetGameId(gameId string) *logrus.Entry {
	log = logger.WithFields(logrus.Fields{
		"gameId": gameId,
	})
	return log
}
