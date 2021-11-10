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

	"gopkg.in/yaml.v3"
)

var (
	logLevel = flag.Int("loglevel", 2, "0 = error, 1 = warn,\n2 = info,  3 = verbose,\n4 = debug")
)

// Config is the config for Release-Notifier.
type Config struct {
	Defaults Defaults     `yaml:"defaults"` // Default values for the various parameters.
	Monitor  MonitorSlice `yaml:"monitor"`  // The targets to monitor and notify on.
}

// Defaults is the global default for vars.
type Defaults struct {
	Service Service `yaml:"service"`
	Slack   Slack   `yaml:"slack"`
	WebHook WebHook `yaml:"webhook"`
}

// setDefaults will set the defaults for each undefined var.
func (d *Defaults) setDefaults() {
	// Service defaults.
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

	// Slack defaults.
	if d.Slack.Delay == "" {
		d.Slack.Delay = "0s"
	}
	if d.Slack.IconEmoji == "" && d.Slack.IconURL == "" {
		d.Slack.IconEmoji = ":github:"
	}
	if d.Slack.MaxTries == 0 {
		d.Slack.MaxTries = 3
	}
	if d.Slack.Message == "" {
		d.Slack.Message = "<${service_url}|${service_id}> - ${version} released"
	}
	if d.Slack.Username == "" {
		d.Slack.Username = "Release Notifier"
	}

	// WebHook defaults.
	if d.WebHook.Delay == "" {
		d.WebHook.Delay = "0s"
	}
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

// SetLogLevel will set logLevel to value if that's in the acceptable range, 2 otherwise
func SetLogLevel(value int) {
	if value > 4 || value < 0 {
		log.Println("ERROR: loglevel should be between 0 and 4 (inclusive), setting yours to 2 (info)")
		*logLevel = 2
	} else {
		*logLevel = value
	}
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
	if *logLevel > 4 || *logLevel < 0 {
		log.Println("ERROR: loglevel should be between 0 and 4 (inclusive), setting yours to 2 (info)")
		*logLevel = 2
	}

	if *logLevel > 2 {
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
		if *logLevel > 1 {
			log.Printf("INFO: %d targets with %d services to monitor:", len(config.Monitor), serviceCount)

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
		}
	}

	// Track all targets for changes in version and act on any
	// found changes.
	(&config).Monitor.track()

	select {}
}
