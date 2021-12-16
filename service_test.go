package main

import (
	"regexp"
	"testing"
)

func TestServiceQuery(t *testing.T) {
	var (
		config     Config
		configFile string = "config.yml.test"
		want              = regexp.MustCompile(`^[0-9.]+[0-9]$`)
	)
	config.getConf(configFile)
	config.setDefaults()

	config.Monitor[2].Service[0].AccessToken = ""
	_ = config.Monitor[2].Service[0].query(0, config.Monitor[2].ID)
	got := config.Monitor[2].Service[0].status.version

	if !want.MatchString(got) {
		t.Fatalf(`%s.status.version = %v, want match for %s`, config.Monitor[1].Service[0].ID, got, want)
	}
}
