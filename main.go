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
	"os"

	"gopkg.in/yaml.v3"
)

var (
	jLog JLog
)

// Config is the config for Release-Notifier.
type Config struct {
	Defaults Defaults     `yaml:"defaults"` // Default values for the various parameters.
	Monitor  MonitorSlice `yaml:"monitor"`  // The targets to monitor and notify on.
}

// Defaults is the global default for vars.
type Defaults struct {
	Gotify  Gotify  `yaml:"gotify"`
	Service Service `yaml:"service"`
	Slack   Slack   `yaml:"slack"`
	WebHook WebHook `yaml:"webhook"`
}

// setDefaults sets undefined variables to their default.
func (d *Defaults) setDefaults() {
	// Service defaults.
	d.Service.AllowInvalidCerts = stringBool(d.Service.AllowInvalidCerts, "", "", false)
	d.Service.IgnoreMiss = stringBool(d.Service.IgnoreMiss, "", "", false)
	d.Service.Interval = valueOrValueString(d.Service.Interval, "10m")
	d.Service.ProgressiveVersioning = stringBool(d.Service.ProgressiveVersioning, "", "", true)
	d.Service.checkValues("defaults", 0, true)

	// Gotify defaults.
	d.Gotify.Delay = valueOrValueString(d.Gotify.Delay, "0s")
	d.Gotify.MaxTries = valueOrValueUInt(d.Gotify.MaxTries, 3)
	d.Gotify.Message = valueOrValueString(d.Gotify.Message, "${service_id} - ${version} released")
	d.Gotify.Priority = valueOrValueString(d.Gotify.Priority, "5")
	d.Gotify.Title = valueOrValueString(d.Gotify.Title, "Release notifier")
	d.Gotify.checkValues("defaults", 0, true)

	// Slack defaults.
	d.Slack.Delay = valueOrValueString(d.Slack.Delay, "0s")
	if d.Slack.IconEmoji == "" && d.Slack.IconURL == "" {
		d.Slack.IconEmoji = ":github:"
	}
	d.Slack.MaxTries = valueOrValueUInt(d.Slack.MaxTries, 3)
	d.Slack.Message = valueOrValueString(d.Slack.Message, "<${service_url}|${service_id}> - ${version} released")
	d.Slack.Username = valueOrValueString(d.Slack.Username, "Release Notifier")
	d.Slack.checkValues("defaults", 0, true)

	// WebHook defaults.
	d.WebHook.Delay = valueOrValueString(d.WebHook.Delay, "0s")
	d.WebHook.MaxTries = valueOrValueUInt(d.WebHook.MaxTries, 3)
	d.WebHook.DesiredStatusCode = valueOrValueInt(d.WebHook.DesiredStatusCode, 0)
	d.WebHook.SilentFails = stringBool(d.WebHook.SilentFails, "", "", false)
	d.WebHook.checkValues("defaults", 0, true)
}

// print will print the defaults
func (d *Defaults) print() {
	fmt.Println("defaults:")

	// Service defaults.
	fmt.Println("  service:")
	fmt.Printf("    allow_invalid_certs: %s\n", d.Service.AllowInvalidCerts)
	fmt.Printf("    ignore_miss: %s\n", d.Service.IgnoreMiss)
	fmt.Printf("    interval: %s\n", d.Service.Interval)
	fmt.Printf("    progressive_versioning: %s\n", d.Service.ProgressiveVersioning)

	// Gotify defaults.
	fmt.Println("  gotify:")
	fmt.Printf("    delay: %s\n", d.Gotify.Delay)
	fmt.Printf("    max_tries: %d\n", d.Gotify.MaxTries)
	fmt.Printf("    message: '%s'\n", d.Gotify.Message)
	fmt.Printf("    priority: %s\n", d.Gotify.Priority)
	fmt.Printf("    title: '%s'\n", d.Gotify.Title)
	if d.Gotify.Extras != (GotifyExtras{}) {
		fmt.Println("    extras:")
		if d.Gotify.Extras.AndroidAction != "" {
			fmt.Printf("      android_action: '%s'\n", d.Gotify.Extras.AndroidAction)
		}
		if d.Gotify.Extras.ClientDisplay != "" {
			fmt.Printf("      client_display: '%s'\n", d.Gotify.Extras.ClientDisplay)
		}
		if d.Gotify.Extras.ClientNotification != "" {
			fmt.Printf("      client_notification: '%s'\n", d.Gotify.Extras.ClientNotification)
		}
	}

	// Slack defaults.
	fmt.Println("  slack:")
	fmt.Printf("    delay: %s\n", d.Slack.Delay)
	fmt.Printf("    icon_emoji: '%s'\n", d.Slack.IconEmoji)
	fmt.Printf("    icon_url: '%s'\n", d.Slack.IconURL)
	fmt.Printf("    max_tries: %d\n", d.Slack.MaxTries)
	fmt.Printf("    message: '%s'\n", d.Slack.Message)
	fmt.Printf("    username: '%s'\n", d.Slack.Username)

	// WebHook defaults.
	fmt.Println("  webhook:")
	fmt.Printf("    delay: %s\n", d.WebHook.Delay)
	fmt.Printf("    desired_status_code: %d\n", d.WebHook.DesiredStatusCode)
	fmt.Printf("    max_tries: %d\n", d.WebHook.MaxTries)
	fmt.Printf("    silent_fails: %s\n", d.WebHook.SilentFails)
}

// getConf reads file as Config.
func (c *Config) getConf(file string) *Config {
	data, err := ioutil.ReadFile(file)
	msg := fmt.Sprintf("Error reading '%s'\n%s ", file, err)
	jLog.Fatal(msg, err != nil)

	err = yaml.Unmarshal(data, c)
	msg = fmt.Sprintf("Unmarshal of '%s' failed\n%s", file, err)
	jLog.Fatal(msg, err != nil)
	return c
}

// setDefaults sets undefined variables to their default.
func (c *Config) setDefaults() *Config {
	c.Defaults.setDefaults()
	for monitorIndex := range c.Monitor {
		monitor := &c.Monitor[monitorIndex]
		monitor.Service.setDefaults(monitor.ID, c.Defaults)
		monitor.Gotify.setDefaults(monitor.ID, c.Defaults)
		monitor.Slack.setDefaults(monitor.ID, c.Defaults)
		monitor.WebHook.setDefaults(monitor.ID, c.Defaults)
	}
	return c
}

// print will print the parsed config.
func (c *Config) print() {
	c.Monitor.print()
	fmt.Println()
	c.Defaults.print()
}

// configPrint will act on the 'config-check' flag and print the parsed
func configPrint(flag *bool, cfg *Config) {
	if *flag {
		cfg.print()
		os.Exit(0)
	}
}

// main loads the config and then calls Monitor.Track to monitor
// each Service of the monitor targets for version changes and act
// on them as defined.
func main() {
	var (
		config          Config
		configFile      = flag.String("config", "config.yml", "The path to the config file to use") // "path/to/config.yml"
		configPrintFlag = flag.Bool("config-check", false, "Use to print the fully-parsed config")
		logLevel        = flag.Int("loglevel", 2, "0 = error, 1 = warn,\n2 = info,  3 = verbose,\n4 = debug")
		timestamps      = flag.Bool("timestamps", false, "Use to enable timestamps in cli output")
	)

	flag.Parse()

	jLog.SetTimestamps(*timestamps)
	jLog.SetLevel(*logLevel)
	msg := fmt.Sprintf("Loading config from '%s'", *configFile)
	jLog.Verbose(msg, true)

	config.getConf(*configFile)
	config.setDefaults()

	// configPrint
	configPrint(configPrintFlag, &config)

	serviceCount := 0
	for mIndex, monitor := range config.Monitor {
		serviceCount += len(monitor.Service)
		for sIndex := range monitor.Service {
			config.Monitor[mIndex].Service[sIndex].status.init()
		}
	}

	if serviceCount == 0 {
		msg := fmt.Sprintf("Exiting as no services to monitor were found in '%s'", *configFile)
		jLog.Error(msg, true)
		os.Exit(1)
	}

	// INFO or above
	if jLog.Level > 1 {
		msg := fmt.Sprintf("%d targets with %d services to monitor:", len(config.Monitor), serviceCount)
		jLog.Info(msg, true)

		for _, monitor := range config.Monitor {
			if len(monitor.Service) == 1 {
				fmt.Printf("  - %s\n", monitor.Service[0].ID)
			} else {
				fmt.Printf("  - %s:\n", monitor.ID)
				for _, service := range monitor.Service {
					fmt.Printf("      - %s\n", service.ID)
				}
			}
		}
	}

	// Track all targets for changes in version and act on any
	// found changes.
	(&config).Monitor.track(config.Defaults)

	select {}
}
