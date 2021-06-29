/*
Release-Notifier monitors GitHub and/or other URLs for version changes.
On a version change, send Slack message(s) and/or webhook(s).
main.go uses track.go for the goroutines that call query.go
and then, on a version change, will call slack.go and webhook.go.
*/
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	verbose = flag.Bool("verbose", false, "Toggle verbose logging")
)

// Config is the config for Release-Notifier.
type Config struct {
	Defaults Defaults     `yaml:"defaults"` // Default values for the various parameters.
	Services ServiceSlice `yaml:"services"` // The services to monitor and notify.
}

// Defaults is the global default for vars.
type Defaults struct {
	Monitor MonitorDefaults `yaml:"monitor"`
	Slack   SlackDefaults   `yaml:"slack"`
	WebHook WebHookDefaults `yaml:"webhook"`
}

// MonitorDefaults are the defaults for Monitor.
type MonitorDefaults struct {
	AccessToken string `yaml:"access_token"` // GitHub access token.
	Interval    int    `yaml:"interval"`     // Interval (in seconds) between each version check.
}

// SlackDefaults are the defaults for Slack.
type SlackDefaults struct {
	IconEmoji string `yaml:"slack_icon_emoji"` // Icon emoji to use for the Slack message.
	Username  string `yaml:"username"`         // Username to send the Slack message as.
	Message   string `yaml:"message"`          // Slack message to send.
	MaxTries  int    `yaml:"maxtries"`         // Number of times to attempt sending the Slack message until a 200 is received.
}

// WebHookDefaults are the defaults for webhook.
type WebHookDefaults struct {
	DesiredStatusCode int    `yaml:"desired_status_code"` // Re-send each WebHook until we get this status code. (0 = accept all 2** codes)
	MaxTries          int    `yaml:"maxtries"`            // Number of times to attempt sending the WebHook if the desired status code is not received.
	SilentFails       string `yaml:"silent_fails"`        // Whether to notify if a WebHook fails MaxTries times
}

// setDefaults will set the defaults for each undefined var.
func (d *Defaults) setDefaults() {
	// MonitorDefaults defaults.
	if d.Monitor.Interval == 0 {
		d.Monitor.Interval = 600
	}

	// SlackDefaults defaults.
	if d.Slack.Message == "" {
		d.Slack.Message = "<${monitor_url}|${monitor_id}> - ${version} released"
	}
	if d.Slack.Username == "" {
		d.Slack.Username = "Release Notifier"
	}
	if d.Slack.IconEmoji == "" {
		d.Slack.IconEmoji = ":github:"
	}
	if d.Slack.MaxTries == 0 {
		d.Slack.MaxTries = 3
	}

	// WebHookDefaults defaults.
	if d.WebHook.DesiredStatusCode == 0 {
		d.WebHook.DesiredStatusCode = 0
	}
	if d.WebHook.MaxTries == 0 {
		d.WebHook.MaxTries = 3
	}
	if strings.ToLower(d.WebHook.SilentFails) == "true" {
		d.WebHook.SilentFails = "y"
	} else {
		d.WebHook.SilentFails = "n"
	}
}

// getConf reads file as Config.
func (c *Config) getConf(file string) *Config {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Printf("ERROR: data.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatalf("ERROR: Unmarshal: %v", err)
	}
	return c
}

// setDefaults sets the defaults for each undefined var.
func (c *Config) setDefaults() *Config {
	c.Defaults.setDefaults()
	for serviceIndex := range c.Services {
		service := &c.Services[serviceIndex]
		service.Monitor.setDefaults(c.Defaults)
		service.Slack.setDefaults(c.Defaults)
		service.WebHook.setDefaults(c.Defaults)
	}
	return c
}

// main loads the config and then calls Service.Track to monitor
// each Service for version changes and act on them as defined.
func main() {
	var (
		configFile = flag.String("config", "config.yml", "The path to the config file to use") // "path/to/config.yml"
		config     Config
	)

	flag.Parse()

	if *verbose {
		log.Printf("INFO: Loading config from %s", *configFile)
	}
	config.getConf(*configFile)
	config.setDefaults()

	sites := ""
	for _, service := range config.Services {
		for _, monitor := range service.Monitor {
			sites = fmt.Sprintf("%s, %s", sites, monitor.ID)
		}
	}
	log.Printf("INFO: %d sites to monitor:", strings.Count(sites, ","))
	if len(sites) != 0 {
		log.Printf("INFO: %s", sites[2:])
	}

	// Trak all services for changes and act on any
	// found changes.
	(&config).Services.track()

	select {}
}
