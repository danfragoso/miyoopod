package main

import (
	"os"
	"time"
)

const LOG_PATH = "./miyoopod.log"

var logFile *os.File

func init() {
	var err error
	logFile, err = os.OpenFile(LOG_PATH, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
}

func logMsg(message string) {
	logFile.WriteString(time.Now().Format("2006-01-02 15:04:05.999") + " - " + message + "\n")
}
