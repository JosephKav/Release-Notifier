package main

import (
	"testing"
)

func TestGetConf(t *testing.T) {
	var (
		config                    Config
		configFile                string = "config.yml.test"
		wantServiceInterval       string = "123"
		wantServiceIntervalParsed string = wantServiceInterval + "s"
		wantSlackDelay            string = "1"
		wantSlackDelayParsed      string = wantSlackDelay + "s"
		wantWebHookDelay          string = "2"
		wantWebHookDelayParsed    string = wantWebHookDelay + "s"
		wantMonitorServiceURL     string = "JosephKav/Release-Notifier"
	)
	config.getConf(configFile)

	// Defaults: Service
	gotServiceInterval := config.Defaults.Service.Interval
	if !(wantServiceInterval == gotServiceInterval) {
		t.Fatalf(`config.Defaults.Service.Interval = %v, want match for %s`, gotServiceInterval, wantServiceInterval)
	}
	config.Defaults.Service.checkValues("defaults", 0, true)
	gotServiceIntervalParsed := config.Defaults.Service.Interval
	if !(wantServiceIntervalParsed == gotServiceIntervalParsed) {
		t.Fatalf(`config.Defaults.Service.Interval = %v, want match for %s`, gotServiceIntervalParsed, wantServiceInterval)
	}

	// Defaults: Slack
	gotSlackDelay := config.Defaults.Slack.Delay
	if !(wantSlackDelay == gotSlackDelay) {
		t.Fatalf(`config.Defaults.Slack.Delay = %s, want match for %s`, gotSlackDelay, wantSlackDelay)
	}
	config.Defaults.Slack.checkValues("defaults", 0, true)
	gotSlackDelayParsed := config.Defaults.Slack.Delay
	if !(wantSlackDelayParsed == gotSlackDelayParsed) {
		t.Fatalf(`config.Defaults.Service.Interval = %v, want match for %s`, gotSlackDelayParsed, wantSlackDelay)
	}

	// Defaults: WebHook
	gotWebHookDelay := config.Defaults.WebHook.Delay
	if !(wantWebHookDelay == gotWebHookDelay) {
		t.Fatalf(`config.Defaults.WebHook.Delay = %s, want match for %s`, gotWebHookDelay, wantWebHookDelay)
	}
	config.Defaults.WebHook.checkValues("defaults", 0, true)
	gotWebHookDelayParsed := config.Defaults.WebHook.Delay
	if !(wantWebHookDelayParsed == gotWebHookDelayParsed) {
		t.Fatalf(`config.Defaults.Service.Interval = %v, want match for %s`, gotWebHookDelayParsed, wantWebHookDelay)
	}

	// Monitor: Service
	gotMonitorServiceURL := config.Monitor[0].Service[0].URL
	if !(wantMonitorServiceURL == gotMonitorServiceURL) {
		t.Fatalf(`config.Monitor[0].Service[0].URL = %s, want match for %s`, gotMonitorServiceURL, wantMonitorServiceURL)
	}
}
func TestSetDefaults(t *testing.T) {
	var (
		config     Config
		configFile string = "config.yml.test"
		wantString string
	)
	config.getConf(configFile)
	wantString = "false"
	gotString := config.Defaults.WebHook.SilentFails
	if !(wantString == gotString) {
		t.Fatalf(`pre-setDefaults config.Defaults.WebHook.SilentFails = %v, want match for %s`, gotString, wantString)
	}

	config.setDefaults()
	wantString = "n"
	gotString = config.Defaults.WebHook.SilentFails
	if !(wantString == gotString) {
		t.Fatalf(`post-setDefaults config.Defaults.WebHook.SilentFails = %v, want match for %s`, gotString, wantString)
	}
}
