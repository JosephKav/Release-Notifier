package main

import (
	"fmt"
	"log"
	"os"
)

// logLevel
// 0 = error  1 = warn,
// 2 = info,  3 = verbose,
// 4 = debug

// logError will error log the msg.
//
// (if otherCondition is true)
func logError(msg string, otherCondition bool) {
	if otherCondition {
		if *timestamps {
			log.Printf("ERROR %s\n", msg)
		} else {
			fmt.Printf("ERROR: %s\n", msg)
		}
	}
}

// logWarn will warning log msg if logLevel is warning (1) or below.
//
// (if otherCondition is true)
func logWarn(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 0 && otherCondition {
		if *timestamps {
			log.Printf("WARNING: %s\n", msg)
		} else {
			fmt.Printf("WARNING: %s\n", msg)
		}
	}
}

// logInfo will info log msg if logLevel is info (2) or above.
//
// (if otherCondition is true)
func logInfo(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 1 && otherCondition {
		if *timestamps {
			log.Printf("INFO: %s\n", msg)
		} else {
			fmt.Printf("INFO: %s\n", msg)
		}
	}
}

// logVerbose will verbose log msg if logLevel is verbose (3) or above.
//
// (if otherCondition is true)
func logVerbose(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 2 && otherCondition {
		if *timestamps {
			log.Printf("VERBOSE: %s\n", msg)
		} else {
			fmt.Printf("VERBOSE: %s\n", msg)
		}
	}
}

// logDebug will debug log msg if logLevel is debug (4).
//
// (if otherCondition is true)
func logDebug(loglevel int, msg string, otherCondition bool) {
	if *logLevel == 4 && otherCondition {
		if *timestamps {
			log.Printf("DEBUG: %s\n", msg)
		} else {
			fmt.Printf("DEBUG: %s\n", msg)
		}
	}
}

// logFatal is equivalent to logError() followed by a call to os.Exit(1).
func logFatal(msg string, otherCondition bool) {
	if otherCondition {
		logError(msg, true)
		os.Exit(1)
	}
}
