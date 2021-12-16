package main

import (
	"fmt"
	"log"
	"os"
)

// JLog is a for various levels of logging.
//
// It supports ERROR, WARNING, INFO, VERBOSE and DEBUG
type JLog struct {
	// Level is the level of the Log
	// 0 = ERROR  1 = WARNING,
	// 2 = INFO,  3 = VERBOSE,
	// 4 = DEBUG
	Level uint
	// Timestamps is whether to log timestamps with the msg, or just the msg.
	Timestamps bool
}

// SetLevel will set the level of the Log.
//
// If value is out of the range (<0 or >4), then exit
func (l *JLog) SetLevel(value int) {
	if value > 4 || value < 0 {
		msg := fmt.Sprintf("loglevel should be between 0 and 4 (inclusive). %d is outside this range.", value)
		l.Fatal(msg, true)
		l.Level = 2
	} else {
		l.Level = uint(value)
	}
}

// SetTimestamps will enable/disable timestamps on the logs.
func (l *JLog) SetTimestamps(enable bool) {
	l.Timestamps = enable
}

// Error will ERROR log the msg.
//
// (if otherCondition is true)
func (l *JLog) Error(msg string, otherCondition bool) {
	if otherCondition {
		if l.Timestamps {
			log.Printf("ERROR %s\n", msg)
		} else {
			fmt.Printf("ERROR: %s\n", msg)
		}
	}
}

// Warn will WARNING log msg if l.Level is > 0 (WARNING, INFO, VERBOSE or DEBUG).
//
// (if otherCondition is true)
func (l *JLog) Warn(msg string, otherCondition bool) {
	if l.Level > 0 && otherCondition {
		if l.Timestamps {
			log.Printf("WARNING: %s\n", msg)
		} else {
			fmt.Printf("WARNING: %s\n", msg)
		}
	}
}

// Info will INFO log msg if l.Level is > 1 (INFO, VERBOSE or DEBUG).
//
// (if otherCondition is true)
func (l *JLog) Info(msg string, otherCondition bool) {
	if l.Level > 1 && otherCondition {
		if l.Timestamps {
			log.Printf("INFO: %s\n", msg)
		} else {
			fmt.Printf("INFO: %s\n", msg)
		}
	}
}

// Verbose will VERBOSE log msg if l.Level is > 2 (VERBOSE or DEBUG).
//
// (if otherCondition is true)
func (l *JLog) Verbose(msg string, otherCondition bool) {
	if l.Level > 2 && otherCondition {
		if l.Timestamps {
			log.Printf("VERBOSE: %s\n", msg)
		} else {
			fmt.Printf("VERBOSE: %s\n", msg)
		}
	}
}

// Debug will DEBUG log msg if l.Level is 4 (DEBUG).
//
// (if otherCondition is true)
func (l *JLog) Debug(msg string, otherCondition bool) {
	if l.Level == 4 && otherCondition {
		if l.Timestamps {
			log.Printf("DEBUG: %s\n", msg)
		} else {
			fmt.Printf("DEBUG: %s\n", msg)
		}
	}
}

// Fatal is equivalent to Error() followed by a call to os.Exit(1).
func (l *JLog) Fatal(msg string, otherCondition bool) {
	if otherCondition {
		l.Error(msg, true)
		os.Exit(1)
	}
}
