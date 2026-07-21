package main

import (
	"time"

	"github.com/t-l3/logger"
)

type v logger.LoggerValues

func main() {
	log := logger.Logger{Name: "Main"}

	log.Info("Starting sh-gen", v{"time": time.Now().String()})
	log.Debug("debug line", nil)
	log.Info("info line", nil)
	log.Warn("warn line", nil)
	log.Error("error line", nil)
	log.Panic("panic line", nil)
}
