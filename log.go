package main

import "log"

// logLevel
// 0 = error  1 = warn,
// 2 = info,  3 = verbose,
// 4 = debug

// logError will error log the msg.
//
// (if otherCondition is true)
func logError(msg string, otherCondition bool) {
	if otherCondition {
		log.Printf("ERROR: %s", msg)
	}
}

// logWarn will warning log msg if logLevel is warning (1) or below.
//
// (if otherCondition is true)
func logWarn(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 0 && otherCondition {
		log.Printf("WARNING: %s", msg)
	}
}

// logInfo will info log msg if logLevel is info (2) or above.
//
// (if otherCondition is true)
func logInfo(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 1 && otherCondition {
		log.Printf("INFO: %s", msg)
	}
}

// logVerbose will verbose log msg if logLevel is verbose (3) or above.
//
// (if otherCondition is true)
func logVerbose(loglevel int, msg string, otherCondition bool) {
	if *logLevel > 2 && otherCondition {
		log.Printf("VERBOSE: %s", msg)
	}
}

// logDebug will debug log msg if logLevel is debug (4).
//
// (if otherCondition is true)
func logDebug(loglevel int, msg string, otherCondition bool) {
	if *logLevel == 4 && otherCondition {
		log.Printf("DEBUG: %s", msg)
	}
}
