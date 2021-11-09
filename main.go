/*
Release-Notifier monitors GitHub and/or other URLs for version changes.
On a version change, send Slack message(s) and/or webhook(s).
main.go uses track.go for the goroutines that call query.go
and then, on a version change, will call slack.go and webhook.go.
*/
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	verbose = flag.Bool("verbose", false, "Toggle verbose logging")
	debug   = flag.Bool("debug", false, "Toggle debug logging")
)

// Config is the config for Release-Notifier.
type Config struct {
	Defaults Defaults     `yaml:"defaults"` // Default values for the various parameters.
	Monitor  MonitorSlice `yaml:"monitor"`  // The targets to monitor and notify on.
}

// Defaults is the global default for vars.
type Defaults struct {
	Service ServiceDefaults `yaml:"service"`
	Slack   SlackDefaults   `yaml:"slack"`
	WebHook WebHookDefaults `yaml:"webhook"`
}

// ServiceDefaults are the defaults for Service.
type ServiceDefaults struct {
	AccessToken           string `yaml:"access_token"`           // GitHub access token.
	AllowInvalidCerts     string `yaml:"allow_invalid"`          // Disallows invalid HTTPS certificates.
	ProgressiveVersioning string `yaml:"progressive_versioning"` // Version has to be greater than the previous to trigger Slack(s)/WebHook(s)
	Interval              int    `yaml:"interval"`               // Interval (in seconds) between each version check.
	IgnoreMiss            string `yaml:"ignore_misses"`          // Ignore URLCommands that fail (e.g. split on text that doesn't exist)
}

// SlackDefaults are the defaults for Slack.
type SlackDefaults struct {
	IconEmoji string        `yaml:"icon_emoji"` // Icon emoji to use for the Slack message.
	IconURL   string        `yaml:"icon_url"`   // Icon URL to use for the Slack message.
	Username  string        `yaml:"username"`   // Username to send the Slack message as.
	Message   string        `yaml:"message"`    // Slack message to send.
	Delay     time.Duration `yaml:"delay"`      // The delay before sending the Slack message.
	MaxTries  int           `yaml:"maxtries"`   // Number of times to attempt sending the Slack message until a 200 is received.
}

// WebHookDefaults are the defaults for webhook.
type WebHookDefaults struct {
	DesiredStatusCode int           `yaml:"desired_status_code"` // Re-send each WebHook until we get this status code. (0 = accept all 2** codes).
	Delay             time.Duration `yaml:"delay"`               // The delay before sending the WebHook.
	MaxTries          int           `yaml:"maxtries"`            // Number of times to attempt sending the WebHook if the desired status code is not received.
	SilentFails       string        `yaml:"silent_fails"`        // Whether to notify if a WebHook fails MaxTries times.
}

// setDefaults will set the defaults for each undefined var.
func (d *Defaults) setDefaults() {
	// ServiceDefaults defaults.
	if strings.ToLower(d.Service.AllowInvalidCerts) == "true" || strings.ToLower(d.Service.AllowInvalidCerts) == "yes" {
		d.Service.AllowInvalidCerts = "y"
	} else {
		d.Service.AllowInvalidCerts = "n"
	}
	if d.Service.Interval == 0 {
		d.Service.Interval = 600
	}
	if strings.ToLower(d.Service.ProgressiveVersioning) == "false" || strings.ToLower(d.Service.ProgressiveVersioning) == "no" {
		d.Service.ProgressiveVersioning = "n"
	} else {
		d.Service.ProgressiveVersioning = "y"
	}
	if strings.ToLower(d.Service.IgnoreMiss) == "true" || strings.ToLower(d.Service.IgnoreMiss) == "yes" {
		d.Service.IgnoreMiss = "y"
	} else {
		d.Service.IgnoreMiss = "n"
	}

	// SlackDefaults defaults.
	if d.Slack.Message == "" {
		d.Slack.Message = "<${service_url}|${service_id}> - ${version} released"
	}
	if d.Slack.Username == "" {
		d.Slack.Username = "Release Notifier"
	}
	if d.Slack.IconEmoji == "" && d.Slack.IconURL == "" {
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
	if strings.ToLower(d.WebHook.SilentFails) == "true" || strings.ToLower(d.WebHook.SilentFails) == "yes" {
		d.WebHook.SilentFails = "y"
	} else {
		d.WebHook.SilentFails = "n"
	}
}

// getConf reads file as Config.
func (c *Config) getConf(file string) *Config {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Printf("ERROR: data.Get err\n%v ", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatalf("ERROR: Unmarshal\n%v", err)
	}
	return c
}

// setDefaults sets the defaults for each undefined var.
func (c *Config) setDefaults() *Config {
	c.Defaults.setDefaults()
	for monitorIndex := range c.Monitor {
		monitor := &c.Monitor[monitorIndex]
		monitor.Service.setDefaults(c.Defaults)
		monitor.Service.checkValues(monitor.ID)
		monitor.Slack.setDefaults(c.Defaults)
		monitor.Slack.checkValues(monitor.ID)
		monitor.WebHook.setDefaults(c.Defaults)
		monitor.WebHook.checkValues(monitor.ID)
	}
	return c
}

// valueOrDefault will return value if it's not empty, dflt otherwise.
func valueOrDefault(value string, dflt string) string {
	if value == "" {
		return dflt
	}
	return value
}

// main loads the config and then calls Monitor.Track to monitor
// each Service of the monitor targets for version changes and act
// on them as defined.
func main() {
	var (
		configFile = flag.String("config", "config.yml", "The path to the config file to use") // "path/to/config.yml"
		config     Config
	)

	flag.Parse()

	if *verbose {
		log.Printf("VERBOSE: Loading config from '%s'", *configFile)
	}
	config.getConf(*configFile)
	config.setDefaults()

	serviceCount := 0
	for mIndex, monitor := range config.Monitor {
		serviceCount += len(monitor.Service)
		for sIndex := range monitor.Service {
			config.Monitor[mIndex].Service[sIndex].status.init()
		}
	}

	if serviceCount == 0 {
		log.Printf("ERROR: Exiting as no services to monitor were found in '%s'", *configFile)
		os.Exit(1)
	} else {
		log.Printf("INFO: %d targets with %d services to monitor:", len(config.Monitor), serviceCount)
	}

	for _, monitor := range config.Monitor {
		if len(monitor.Service) == 1 {
			log.Printf("  - %s", monitor.Service[0].ID)
		} else {
			log.Printf("  - %s:", monitor.ID)
			for _, service := range monitor.Service {
				log.Printf("      - %s", service.ID)
			}
		}
	}

	// Track all targets for changes in version and act on any
	// found changes.
	(&config).Monitor.track()

	select {}
}
