package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SlackSlice is an array of Slack's.
type SlackSlice []Slack

// Slack is a Slack message w/ destination and from details.
type Slack struct {
	URL       string `yaml:"url"`      // "https://example.com
	Message   string `yaml:"message"`  // "${monitor} - ${version} released"
	Username  string `yaml:"username"` // "Release Notifier"
	IconEmoji string `yaml:"icon"`     // ":github:"
	MaxTries  int    `yaml:"maxtries"` // Number of times to attempt sending the Slack message if a 200 is not received.
}

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of a dict.
//
// e.g.    Slack: { url: "example.com" }
//
// becomes Slack: [ { url: "example.com" } ]
func (s *SlackSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []Slack
	err := unmarshal(&multi)
	if err != nil {
		var single Slack
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*s = []Slack{single}
	} else {
		*s = multi
	}
	return nil
}

// setDefaults calls setDefaults on each Slack to set the defaults for undefined values.
func (s *SlackSlice) setDefaults(defaults Defaults) {
	for SlackIndex := range *s {
		(*s)[SlackIndex].setDefaults(defaults)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (s *Slack) setDefaults(defaults Defaults) {
	if s.Message == "" {
		s.Message = defaults.Slack.Message
	}

	if s.Username == "" {
		s.Username = defaults.Slack.Username
	}

	if s.IconEmoji == "" {
		s.IconEmoji = defaults.Slack.IconEmoji
	}

	if s.MaxTries == 0 {
		s.MaxTries = defaults.Slack.MaxTries
	}
}

// SlackPayload is the payload to be to be sent as the Slack message.
type SlackPayload struct {
	Username  string `json:"username"`   // "Release Notifier"
	IconEmoji string `json:"icon_emoji"` // ":github:"
	Text      string `json:"text"`       // "${monitor} - ${version} released"
}

// send will send every slack message in this SlackSlice.
func (s *SlackSlice) send(serviceID string, mon *Monitor, message string) {
	for index := range *s {
		// Send each Slack message up to s.MaxTries number of times until they 200
		go func() {
			index := index                    // Create new instance for the goroutine.
			triesLeft := (*s)[index].MaxTries // Number of times to send WebHook (until 200 received).
			for {
				err := (*s)[index].send(serviceID, mon, message)

				// SUCCESS
				if err == nil {
					return
				}
				log.Printf("ERROR: Sending Slack failed.\n%v", err)

				// FAIL
				triesLeft--

				// Give up after MaxTries
				if triesLeft == 0 {
					// If not verbose (this would already have been printed in verbose)
					if !*verbose {
						log.Printf("ERROR: %s", err)
					}
					log.Printf("ERROR: %s, Failed %d times to send a slack message to %s", serviceID, (*s)[index].MaxTries, (*s)[index].URL)
					return
				}

				// Space out retries
				time.Sleep(10 * time.Second)
			}
		}()
		// Space out Slack messages
		time.Sleep(3 * time.Second)
	}
}

// send sends a formatted Slack notification regarding m.
func (s *Slack) send(serviceID string, mon *Monitor, message string) error {
	mURL := mon.URL
	// GitHub monitor. Get the non-API URL.
	if mon.Type == "github" {
		mURL = strings.Split(mon.URL, "github.com/repos/")[1]
		mURL = fmt.Sprintf("https://github.com/%s/%s", strings.Split(mURL, "/")[0], strings.Split(mURL, "/")[1])
	}

	// Use default new release Slack message (Not a custom message)
	if message == "" {
		message = s.Message
		message = strings.ReplaceAll(message, "${service}", serviceID)
		message = strings.ReplaceAll(message, "${monitor_url}", mURL)
		message = strings.ReplaceAll(message, "${monitor_id}", mon.ID)
		message = strings.ReplaceAll(message, "${version}", mon.status.version)
	}

	payload := SlackPayload{
		Username:  s.Username,
		IconEmoji: s.IconEmoji,
		Text:      message,
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.URL, bytes.NewReader(payloadData))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// SUCCESS (2XX)
	if strconv.Itoa(resp.StatusCode)[:1] == "2" {
		log.Printf("INFO: %s, Slack message sent", serviceID)
		return nil
	}

	// FAIL
	body, _ := ioutil.ReadAll(resp.Body)
	if *verbose {
		log.Printf("ERROR: Slack request didn't 2XX:\n%s\n%s", resp.Status, body)
	}
	return fmt.Errorf("%s. %s", resp.Status, body)
}
