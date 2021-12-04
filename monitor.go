package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// MonitorSlice is an array of Monitor.
type MonitorSlice []Monitor

// Monitor is a target to monitor along with the actions to take
// when a new release is found for one if its services.
type Monitor struct {
	ID      string       `yaml:"id"`      // "SERVICE_NAME"
	Service ServiceSlice `yaml:"service"` // The service(s) to monitor.
	WebHook WebHookSlice `yaml:"webhook"` // WebHook(s) to send on a new release.
	Slack   SlackSlice   `yaml:"slack"`   // Slack message(s) to send on a new release.
}

// print will print the Monitor's in the MonitorSlice
func (m *MonitorSlice) print() {
	fmt.Println("monitor:")
	for _, monitor := range *m {
		monitor.print()
	}
}

// print will print the Monitor
func (m *Monitor) print() {
	fmt.Printf("  - id: %s\n", m.ID)
	// Service.
	fmt.Println("    service:")
	for _, service := range m.Service {
		fmt.Printf("      - id: %s\n", service.ID)
		fmt.Printf("        type: %s\n", service.Type)
		fmt.Printf("        url: '%s'\n", service.URL)
		service.URLCommands.print("        ")
		fmt.Printf("        interval: %s\n", service.Interval)
		fmt.Printf("        regex_content: %s\n", service.RegexContent)
		fmt.Printf("        regex_version: %s\n", service.RegexVersion)
		fmt.Printf("        progressive_versioning: %s\n", service.ProgressiveVersioning)
		fmt.Printf("        skip_slack: %t\n", service.SkipSlack)
		fmt.Printf("        skip_webhook: %t\n", service.SkipWebHook)
		fmt.Printf("        access_token: '%s'\n", service.AccessToken)
		fmt.Printf("        allow_invalid: %s\n", service.AllowInvalidCerts)
		fmt.Printf("        ignore_misses: %s\n", service.IgnoreMiss)
	}
	// Slack.
	fmt.Println("    slack:")
	for _, slack := range m.Slack {
		fmt.Printf("      - url: '%s'\n", slack.URL)
		fmt.Printf("        icon_emoji: '%s'\n", slack.IconEmoji)
		fmt.Printf("        icon_url: '%s'\n", slack.IconURL)
		fmt.Printf("        username: '%s'\n", slack.Username)
		fmt.Printf("        message: '%s'\n", slack.Message)
		fmt.Printf("        delay: %s\n", slack.Delay)
		fmt.Printf("        max_tries: %d\n", slack.MaxTries)
	}

	// WebHook.
	fmt.Println("    wwbhook:")
	for _, webhook := range m.WebHook {
		fmt.Printf("      - type: %s\n", webhook.Type)
		fmt.Printf("        url: '%s'\n", webhook.URL)
		fmt.Printf("        secret: '%s'\n", webhook.Secret)
		fmt.Printf("        desired_status_code: %d\n", webhook.DesiredStatusCode)
		fmt.Printf("        delay: %s\n", webhook.Delay)
		fmt.Printf("        max_tries: %d\n", webhook.MaxTries)
		fmt.Printf("        silent_fails: %s\n", webhook.SilentFails)
	}
}

// track will track each Monitor (in the MonitorSlice) in this ServiceSlice
// in their own goroutines.
func (m *MonitorSlice) track() {
	// Loop through each service.
	for monitorIndex := range *m {
		for serviceIndex := range (*m)[monitorIndex].Service {
			if *logLevel > 2 {
				log.Printf("VERBOSE: Tracking %s at %s every %s", (*m)[monitorIndex].Service[serviceIndex].ID, (*m)[monitorIndex].Service[serviceIndex].URL, (*m)[monitorIndex].Service[serviceIndex].Interval)
			}

			// Track this Service in a infinite loop goroutine.
			go (*m)[monitorIndex].track(serviceIndex)

			// Space out the tracking of each Service.
			time.Sleep(time.Duration(rand.Intn(10)+10) * time.Second)
		}
	}
}

// Track will track the Monitor.Service data and then send Slack
// messages (Monitor.Slack) as well as WebHooks (Monitor.WebHook)
// when a new release is spottem. It sleeps for Monitor.Interval
// between each check.
func (m *Monitor) track(serviceIndex int) {
	// Track forever.
	for {
		// If new release found by this query.
		if m.Service[serviceIndex].query(serviceIndex, m.ID) {
			if !m.Service[serviceIndex].SkipSlack {
				// Send the Slack Message(s).
				go m.Slack.send(m.ID, &m.Service[serviceIndex], "")
			}

			if !m.Service[serviceIndex].SkipWebHook {
				// Send the WebHook(s).
				go m.WebHook.send(m.ID, m.Service[serviceIndex].ID, m.Slack)
			}
		}

		// Sleep interval between checks.
		sleepTime, _ := time.ParseDuration(m.Service[serviceIndex].Interval)
		time.Sleep(sleepTime)
	}
}
