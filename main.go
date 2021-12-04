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
	"os"

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

// setDefaults sets undefined variables to their default.
func (d *Defaults) setDefaults() {
	// Service defaults.
	d.Service.AllowInvalidCerts = stringBool(d.Service.AllowInvalidCerts, "", "", false)
	d.Service.IgnoreMiss = stringBool(d.Service.IgnoreMiss, "", "", false)
	d.Service.Interval = valueOrValueUInt(d.Service.Interval, 600)
	d.Service.ProgressiveVersioning = stringBool(d.Service.ProgressiveVersioning, "", "", true)

	// Slack defaults.
	d.Slack.Delay = valueOrValueString(d.Slack.Delay, "0s")
	if d.Slack.IconEmoji == "" && d.Slack.IconURL == "" {
		d.Slack.IconEmoji = ":github:"
	}
	d.Slack.MaxTries = valueOrValueUInt(d.Slack.MaxTries, 3)
	d.Slack.Message = valueOrValueString(d.Slack.Message, "<${service_url}|${service_id}> - ${version} released")
	d.Slack.Username = valueOrValueString(d.Slack.Username, "Release Notifier")

	// WebHook defaults.
	d.WebHook.Delay = valueOrValueString(d.WebHook.Delay, "0s")
	d.WebHook.DesiredStatusCode = valueOrValueInt(d.WebHook.DesiredStatusCode, 0)
	d.WebHook.MaxTries = valueOrValueUInt(d.WebHook.MaxTries, 3)
	d.WebHook.SilentFails = stringBool(d.WebHook.SilentFails, "", "", false)
}

// print will print the defaults
func (d *Defaults) print() {
	fmt.Println("defaults:")

	// Service defaults.
	fmt.Println("  service:")
	fmt.Printf("    allow_invalid_certs: %s\n", d.Service.AllowInvalidCerts)
	fmt.Printf("    ignore_miss: %s\n", d.Service.IgnoreMiss)
	fmt.Printf("    interval: %d\n", d.Service.Interval)
	fmt.Printf("    progressive_versioning: %s\n", d.Service.ProgressiveVersioning)

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
	if err != nil {
		log.Printf("ERROR: data.Get err\n%v ", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatalf("ERROR: Unmarshal\n%v", err)
	}
	return c
}

// setDefaults sets undefined variables to their default.
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

// print will print the parsed config.
func (c *Config) print() {
	c.Monitor.print()
	fmt.Println()
	c.Defaults.print()
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
		configFile      = flag.String("config", "config.yml", "The path to the config file to use") // "path/to/config.yml"
		configPrintFlag = flag.Bool("config-check", false, "Use to print the fully-parsed config")
		config          Config
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
		log.Printf("ERROR: Exiting as no services to monitor were found in '%s'", *configFile)
		os.Exit(1)
	}

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

	// Track all targets for changes in version and act on any
	// found changes.
	(&config).Monitor.track()

	select {}
}
