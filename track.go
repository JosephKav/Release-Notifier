package main

import (
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

// track will track each Monitor (in the MonitorSlice) in this ServiceSlice
// in their own goroutines.
func (m *MonitorSlice) track() {
	// Loop through each service.
	for monitorIndex := range *m {
		for serviceIndex := range (*m)[monitorIndex].Service {
			if *logLevel > 2 {
				log.Printf("VERBOSE: Tracking %s at %s every %d seconds", (*m)[monitorIndex].Service[serviceIndex].ID, (*m)[monitorIndex].Service[serviceIndex].URL, (*m)[monitorIndex].Service[serviceIndex].Interval)
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
// when a new release is spotted. It sleeps for Monitor.Interval
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
		time.Sleep(time.Duration(m.Service[serviceIndex].Interval) * time.Second)
	}
}
