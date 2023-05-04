package main

import (
	"log"
	"os"
)

type level uint8
type any = interface{}

const (
	INFO level = iota
	DEBUG
	ERROR
)

type logger struct {
	level level
	*log.Logger
}

func newLogger(level level) *logger {
	return &logger{
		level:  level,
		Logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (logger *logger) info(v ...any) {
	if logger.level <= INFO {
		logger.Println(v...)
	}
}

func (logger *logger) infof(format string, v ...any) {
	if logger.level <= INFO {
		logger.Printf(format, v...)
	}
}

func (logger *logger) debug(v ...any) {
	if logger.level <= DEBUG {
		logger.Println(v...)
	}
}

func (logger *logger) error(v ...any) {
	if logger.level <= ERROR {
		logger.Println(v...)
	}
}

func (logger *logger) setLevel(level level) {
	logger.level = level
}
