package main

import (
	"strings"
)

// stringBool hand;es 'a' as a boolean in string form and returns valueIfA or valueIfNotA.
//
// trueIfBlank is for handling whether 'a' being blank returns true (valueIfA), or false (valueIfNotA)
func stringBool(a string, valueIfA string, valueIfNotA string, trueIfBlank bool) string {
	a = strings.ToLower(a)
	if valueIfA == "" {
		valueIfA = "y"
	}
	if valueIfNotA == "" {
		valueIfNotA = "n"
	}

	switch a {
	case "true", "yes", "y":
		return valueIfA
	case "":
		if trueIfBlank {
			return valueIfA
		}
		return valueIfNotA
	}

	return valueIfNotA
}

// valueOrValueString handles string's and returns 'a' if it's not the default (""), otherwise it returns 'b'.
func valueOrValueString(a string, b string) string {
	if a == "" {
		return b
	}
	return a
}

// valueOrValueInt handles int's and returns 'a' if it's not the default (0), otherwise it returns 'b'.
func valueOrValueInt(a int, b int) int {
	if a == 0 {
		return b
	}
	return a
}

// valueOrValueUInt handles uint's and returns 'a' if it's not the default (0), otherwise it returns 'b'.
func valueOrValueUInt(a uint, b uint) uint {
	if a == 0 {
		return b
	}
	return a
}
