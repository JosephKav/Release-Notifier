package main

import (
	"log"
	"math/rand"
	"time"
)

// ServiceSlice is an array of Service.
type ServiceSlice []Service

// Service is a service to monitor along with the actions to take
// when a new release is found.
type Service struct {
	ID      string       `yaml:"id"`      // "SERVICE_NAME"
	Monitor MonitorSlice `yaml:"monitor"` // The source(s) to monitor.
	WebHook WebHookSlice `yaml:"webhook"` // WebHook(s) to send on a new release.
	Slack   SlackSlice   `yaml:"slack"`   // Slack message(s) to send on a new release.
}

// track will track each Monitor (in the MonitorSlice) in this ServiceSlice
// in their own goroutines.
func (s *ServiceSlice) track() {
	// Loop through each service.
	for serviceIndex := range *s {
		for monitorIndex := range (*s)[serviceIndex].Monitor {
			if *verbose {
				log.Printf("VERBOSE: Tracking %s at %s every %d seconds", (*s)[serviceIndex].Monitor[monitorIndex].ID, (*s)[serviceIndex].Monitor[monitorIndex].URL, (*s)[serviceIndex].Monitor[monitorIndex].Interval)
			}

			// Track this Monitor in a infinite loop goroutine.
			go (*s)[serviceIndex].track(monitorIndex)

			// Space out the tracking of each Monitor.
			time.Sleep(time.Duration(rand.Intn(10)+10) * time.Second)
		}
	}
}

// Track will track the Service.Monitor data and then send Slack
// messages (Service.Slack) as well as WebHooks (Service.WebHook)
// when a new release is spotted. It sleeps for Service.Interval
// between each check.
func (s *Service) track(i int) {
	// Track forever.
	for {
		// If new release found by this query.
		if s.Monitor[i].query(i) {
			if !s.Monitor[i].SkipSlack {
				// Send the Slack Messages.
				go s.Slack.send(s.ID, &s.Monitor[i], "")
			}

			if !s.Monitor[i].SkipWebHook {
				// Send the WebHooks.
				go s.WebHook.send(s.ID, &s.Monitor[i], s.Slack)
			}
		}

		// Sleep interval between checks.
		time.Sleep(time.Duration(s.Monitor[i].Interval) * time.Second)
	}
}
