package main

import (
	"testing"
)

func TestGetConf(t *testing.T) {
	var (
		config                        Config
		configFile                    string = "config.yml.test"
		wantWebHookWebHookSilentFails uint   = 123
		wantSlackDelay                string = "1s"
		wantWebHookDelay              string = "2s"
		wantMonitorServiceURL         string = "JosephKav/Release-Notifier"
	)
	config.getConf(configFile)
	gotWebHookWebHookSilentFails := config.Defaults.Service.Interval

	if !(wantWebHookWebHookSilentFails == gotWebHookWebHookSilentFails) {
		t.Fatalf(`config.getConf(*configFile).Defaults.Service.Interval = %v, want match for %d`, gotWebHookWebHookSilentFails, wantWebHookWebHookSilentFails)
	}

	gotSlackDelay := config.Defaults.Slack.Delay
	if !(wantSlackDelay == gotSlackDelay) {
		t.Fatalf(`config.getConf(*configFile).Defaults.Slack.Delay = %s, want match for %s`, gotSlackDelay, wantSlackDelay)
	}

	gotWebHookDelay := config.Defaults.WebHook.Delay
	if !(wantWebHookDelay == gotWebHookDelay) {
		t.Fatalf(`config.getConf(*configFile).Defaults.WebHook.Delay = %s, want match for %s`, gotWebHookDelay, wantWebHookDelay)
	}

	gotMonitorServiceURL := config.Monitor[0].Service[0].URL
	if !(wantMonitorServiceURL == gotMonitorServiceURL) {
		t.Fatalf(`config.getConf(*configFile).Monitor[0].Service[0].URL = %s, want match for %s`, gotMonitorServiceURL, wantMonitorServiceURL)
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
