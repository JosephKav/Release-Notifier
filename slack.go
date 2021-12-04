package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// SlackSlice is an array of Slack.
type SlackSlice []Slack

// Slack is a Slack message w/ destination and from details.
type Slack struct {
	URL       string `yaml:"url,omitempty"`        // "https://example.com
	IconEmoji string `yaml:"icon_emoji,omitempty"` // ":github:"
	IconURL   string `yaml:"icon_url,omitempty"`   // "https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png"
	Username  string `yaml:"username,omitempty"`   // "Release Notifier"
	Message   string `yaml:"message,omitempty"`    // "${service} - ${version} released"
	Delay     string `yaml:"delay,omitempty"`      // The delay before sending the Slack message.
	MaxTries  uint   `yaml:"maxtries,omitempty"`   // Number of times to attempt sending the Slack message if a 200 is not received.
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

// setDefaults sets undefined variables to their default.
func (s *SlackSlice) setDefaults(monitorID string, defaults Defaults) {
	for slackIndex := range *s {
		(*s)[slackIndex].setDefaults(defaults)
	}
	(*s).checkValues(monitorID)
}

// setDefaults sets undefined variables to their default.
func (s *Slack) setDefaults(defaults Defaults) {
	// Delay
	s.Delay = valueOrValueString(s.Delay, defaults.Slack.Delay)

	// Icon
	if s.IconEmoji == "" && s.IconURL == "" {
		// IconEmoji
		s.IconEmoji = valueOrValueString(s.IconEmoji, defaults.Slack.IconEmoji)
		// IconURL
		s.IconURL = valueOrValueString(s.IconURL, defaults.Slack.IconURL)
	}

	// MaxTries
	s.MaxTries = valueOrValueUInt(s.MaxTries, defaults.Slack.MaxTries)

	// Message
	s.Message = valueOrValueString(s.Message, defaults.Slack.Message)

	// Username
	s.Username = valueOrValueString(s.Username, defaults.Slack.Username)
}

// checkValues will check the variables for all of this monitors Slack recipients.
func (s *SlackSlice) checkValues(monitorID string) {
	for index := range *s {
		(*s)[index].checkValues(monitorID, index, len(*s) == 1)
	}
}

// checkValues will check that the variables are valid for this Slack recipient.
func (s *Slack) checkValues(monitorID string, index int, loneService bool) {
	target := monitorID + ".slack"
	if !loneService {
		target = fmt.Sprintf("%s[%d]", monitorID, index)
	}

	// Delay
	if s.Delay != "" {
		// Default to seconds when an integer is provided
		if _, err := strconv.Atoi(s.Delay); err == nil {
			s.Delay += "s"
		}
		if _, err := time.ParseDuration(s.Delay); err != nil {
			fmt.Printf("ERROR: %s.delay (%s) is invalid (Use 'AhBmCs' duration format)", target, s.Delay)
			os.Exit(1)
		}
	}
}

// SlackPayload is the payload to be to be sent as the Slack message.
type SlackPayload struct {
	Username  string `json:"username"`   // "Release Notifier"
	IconEmoji string `json:"icon_emoji"` // ":github:"
	IconURL   string `json:"icon_url"`   // "https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png"
	Text      string `json:"text"`       // "${service} - ${version} released"
}

// send will send every slack message in this SlackSlice.
func (s *SlackSlice) send(monitorID string, svc *Service, message string) {
	for index := range *s {
		// Send each Slack message up to s.MaxTries number of times until they 200.
		go func() {
			index := index                    // Create new instance for the goroutine.
			triesLeft := (*s)[index].MaxTries // Number of times to send WebHook (until 200 received).

			// Delay sending the Slack message by the defined interval.
			sleepTime, _ := time.ParseDuration((*s)[index].Delay)
			msg := fmt.Sprintf("%s, Sleeping for %s before sending the Slack message", monitorID, (*s)[index].Delay)
			logInfo(*logLevel, msg, sleepTime != 0)
			time.Sleep(sleepTime)

			for {
				err := (*s)[index].send(monitorID, svc, message)

				// SUCCESS
				if err == nil {
					return
				}
				log.Printf("ERROR: %s (%s), Sending Slack failed.\n%v", svc.ID, monitorID, err)

				// FAIL
				triesLeft--

				// Give up after MaxTries.
				if triesLeft == 0 {
					// If not verbose or above (above, this would already have been printed).
					msg := fmt.Sprintf("%s", err)
					logError(msg, (*logLevel < 3))
					log.Printf("ERROR: %s (%s), Failed %d times to send a slack message to %s", svc.ID, monitorID, (*s)[index].MaxTries, (*s)[index].URL)
					return
				}

				// Space out retries.
				time.Sleep(10 * time.Second)
			}
		}()
		// Space out Slack messages.const.
		time.Sleep(3 * time.Second)
	}
}

// send sends a formatted Slack notification regarding mon.
func (s *Slack) send(monitorID string, svc *Service, message string) error {
	sURL := svc.URL
	// GitHub monitor. Get the non-API URL.
	if svc.Type == "github" {
		sURL = strings.Split(svc.URL, "github.com/repos/")[1]
		sURL = fmt.Sprintf("https://github.com/%s/%s", strings.Split(sURL, "/")[0], strings.Split(sURL, "/")[1])
	}

	// Use 'new release' Slack message (Not a custom message)
	if message == "" {
		message = valueOrValueString(svc.Slack.Message, s.Message)
		message = strings.ReplaceAll(message, "${monitor_id}", monitorID)
		message = strings.ReplaceAll(message, "${service_url}", sURL)
		message = strings.ReplaceAll(message, "${service_id}", svc.ID)
		message = strings.ReplaceAll(message, "${version}", svc.status.version)
	}

	payload := SlackPayload{
		Username:  valueOrValueString(svc.Slack.Username, s.Username),
		IconEmoji: valueOrValueString(svc.Slack.IconEmoji, s.IconEmoji),
		IconURL:   valueOrValueString(svc.Slack.IconURL, s.IconURL),
		Text:      message,
	}
	// Handle per-monitor overrides. (Ensure s.Icon* values won't be sent)
	if svc.Slack.IconEmoji != "" {
		payload.IconURL = ""
	} else if svc.Slack.IconURL != "" {
		payload.IconEmoji = ""
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
		// If verbose or above, print the error every time
		msg := fmt.Sprintf("%s (%s), Slack\n%s", svc.ID, monitorID, err)
		logVerbose(*logLevel, msg, true)
		return err
	}
	defer resp.Body.Close()

	// SUCCESS (2XX)
	if strconv.Itoa(resp.StatusCode)[:1] == "2" {

		msg := fmt.Sprintf("%s (%s), Slack message sent", svc.ID, monitorID)
		logInfo(*logLevel, msg, true)
		return nil
	}

	// FAIL
	body, _ := ioutil.ReadAll(resp.Body)
	// If verbose or above, print the error every time
	msg := fmt.Sprintf("%s (%s), Slack request didn't 2XX\n%s\n%s", svc.ID, monitorID, resp.Status, body)
	logVerbose(*logLevel, msg, true)
	return fmt.Errorf("%s. %s", resp.Status, body)
}
