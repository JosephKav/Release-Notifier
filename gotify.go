package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GotifyExtras are the message extras (https://gotify.net/docs/msgextras) for the Gotify messages.
type GotifyExtras struct {
	AndroidAction      string `yaml:"android_action"`      // URL to open on notification delivery
	ClientDisplay      string `yaml:"client_display"`      // Render message in 'text/plain' or 'text/markdown'
	ClientNotification string `yaml:"client_notification"` // URL to open on notification click
}

// GotifySlice is an array of Gotify.
type GotifySlice []Gotify

// Gotify is a Gotify message w/ destination and from details.
type Gotify struct {
	URL      string       `yaml:"url,omitempty"`       // "https://example.com
	Token    string       `yaml:"token,omitempty"`     // apptoken
	Title    string       `yaml:"string,omitempty"`    // "${service_id} - ${version} released"
	Message  string       `yaml:"message,omitempty"`   // "Release notifier"
	Extras   GotifyExtras `yaml:"extras,omitempty"`    // Message extras
	Priority string       `yaml:"priority,omitempty"`  // <1 = Min, 1-3 = Low, 4-7 = Med, >7 = High
	Delay    string       `yaml:"delay,omitempty"`     // The delay before sending the Gotify message.
	MaxTries uint         `yaml:"max_tries,omitempty"` // Number of times to attempt sending the Gotify message if a 200 is not received.
}

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of a dict.
//
// e.g.    Gotify: { url: "example.com" }
//
// becomes Gotify: [ { url: "example.com" } ]
func (g *GotifySlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []Gotify
	err := unmarshal(&multi)
	if err != nil {
		var single Gotify
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*g = []Gotify{single}
	} else {
		*g = multi
	}
	return nil
}

// setDefaults sets undefined variables to their default.
func (g *GotifySlice) setDefaults(monitorID string, defaults Defaults) {
	for gotifyIndex := range *g {
		(*g)[gotifyIndex].setDefaults(defaults)
	}
	(*g).checkValues(monitorID)
}

// setDefaults sets undefined variables to their default.
func (g *Gotify) setDefaults(defaults Defaults) {
	// Delay
	g.Delay = valueOrValueString(g.Delay, defaults.Gotify.Delay)

	// MaxTries
	g.MaxTries = valueOrValueUInt(g.MaxTries, defaults.Gotify.MaxTries)

	// Message
	g.Message = valueOrValueString(g.Message, defaults.Gotify.Message)

	// Priority
	g.Priority = valueOrValueString(g.Priority, defaults.Gotify.Priority)

	// Title
	g.Title = valueOrValueString(g.Title, defaults.Gotify.Title)
}

// checkValues will check the variables for all of this monitors Gotify recipients.
func (g *GotifySlice) checkValues(monitorID string) {
	for index := range *g {
		(*g)[index].checkValues(monitorID, index, len(*g) == 1)
	}
}

// checkValues will check that the variables are valid for this Gotify recipient.
func (g *Gotify) checkValues(monitorID string, index int, loneService bool) {
	target := monitorID + ".gotify"
	if !loneService {
		target = fmt.Sprintf("%s[%d]", monitorID, index)
	}

	// Delay
	if g.Delay != "" {
		// Default to seconds when an integer is provided
		if _, err := strconv.Atoi(g.Delay); err == nil {
			g.Delay += "s"
		}
		if _, err := time.ParseDuration(g.Delay); err != nil {
			msg := fmt.Sprintf("%s.delay '%s' is invalid (Use 'AhBmCs' duration format)", target, g.Delay)
			jLog.Fatal(msg, true)
		}
	}

	if _, err := strconv.Atoi(g.Priority); err != nil {
		msg := fmt.Sprintf("%s.priority '%s' is invalid, it should be an integer, not a %T.", target, g.Priority, g.Priority)
		jLog.Fatal(msg, true)
	}
}

// GotifyPayload is the payload to be to be sent as the Gotify message.
type GotifyPayload struct {
	Extras   map[string]interface{} `json:"extras,omitempty"`
	Message  string                 `form:"message" query:"message" json:"message" binding:"required"`
	Priority int                    `json:"priority"`
	Title    string                 `form:"title" query:"title" json:"title"`
}

// HandleExtras will parse the messaging extras from 'extras' and 'defaults' into the GotifyPayload.
func (p *GotifyPayload) HandleExtras(extras GotifyExtras, defaults GotifyExtras, serviceURL string) {
	// When received on Android and Gotify app is in focus
	androidAction := valueOrValueString(extras.AndroidAction, defaults.AndroidAction)
	if androidAction != "" {
		p.Extras["android::action"] = map[string]interface{}{
			"onReceive": map[string]string{
				"intentUrl": strings.ReplaceAll(androidAction, "${service_url}", serviceURL),
			},
		}
	}

	// Fomatting (markdown / plain)
	clientDisplay := valueOrValueString(extras.ClientDisplay, defaults.ClientDisplay)
	if clientDisplay != "" {
		p.Extras["client::display"] = map[string]interface{}{
			"click": map[string]string{
				"url": strings.ReplaceAll(clientDisplay, "${service_url}", serviceURL),
			},
		}
	}

	// When the notification is clicked (Android)
	clientNotification := valueOrValueString(extras.ClientNotification, defaults.ClientNotification)
	if clientNotification != "" {
		p.Extras["client::notification"] = map[string]interface{}{
			"click": map[string]string{
				"url": strings.ReplaceAll(clientNotification, "${service_url}", serviceURL),
			},
		}
	}
}

// send will send every gotify message in this GotifySlice.
func (g *GotifySlice) send(monitorID string, svc *Service, title string, message string, defaults Gotify) {
	for index := range *g {
		// Send each Gotify message up to s.MaxTries number of times until they 200.
		go func() {
			index := index                    // Create new instance for the goroutine.
			triesLeft := (*g)[index].MaxTries // Number of times to send WebHook (until 200 received).

			// Delay sending the Gotify message by the defined interval.
			sleepTime, _ := time.ParseDuration((*g)[index].Delay)
			msg := fmt.Sprintf("%s, Sleeping for %s before sending the Gotify message", monitorID, (*g)[index].Delay)
			jLog.Info(msg, sleepTime != 0)
			time.Sleep(sleepTime)

			for {
				err := (*g)[index].send(monitorID, svc, title, message, defaults)

				// SUCCESS!
				if err == nil {
					return
				}

				// FAIL
				jLog.Error(err.Error(), true)
				triesLeft--

				// Give up after MaxTries.
				if triesLeft == 0 {
					msg = fmt.Sprintf("%s (%s), Failed %d times to send a Gotify message to %s", svc.ID, monitorID, (*g)[index].MaxTries, (*g)[index].URL)
					jLog.Error(msg, true)
					return
				}

				// Space out retries.
				time.Sleep(10 * time.Second)
			}
		}()
		// Space out Gotify messages.const.
		time.Sleep(3 * time.Second)
	}
}

// send sends a formatted Gotify notification regarding mon.
func (g *Gotify) send(monitorID string, svc *Service, title string, message string, defaults Gotify) error {
	serviceURL := svc.URL
	// GitHub monitor. Get the non-API URL.
	if svc.Type == "github" {
		serviceURL = strings.Split(svc.URL, "github.com/repos/")[1]
		serviceURL = fmt.Sprintf("https://github.com/%s/%s", strings.Split(serviceURL, "/")[0], strings.Split(serviceURL, "/")[1])
	}

	// Use 'new release' Gotify message (Not a custom message)
	if message == "" {
		message = valueOrValueString(svc.Gotify.Message, g.Message)
		message = strings.ReplaceAll(message, "${monitor_id}", monitorID)
		message = strings.ReplaceAll(message, "${service_url}", serviceURL)
		message = strings.ReplaceAll(message, "${service_id}", svc.ID)
		message = strings.ReplaceAll(message, "${version}", svc.status.version)

		title = valueOrValueString(svc.Gotify.Title, g.Title)
		title = strings.ReplaceAll(title, "${monitor_id}", monitorID)
		title = strings.ReplaceAll(title, "${service_url}", serviceURL)
		title = strings.ReplaceAll(title, "${service_id}", svc.ID)
		title = strings.ReplaceAll(title, "${version}", svc.status.version)
	}

	gotifyURL := fmt.Sprintf("%s/message?token=%s", g.URL, g.Token)

	priority, _ := strconv.Atoi(g.Priority)
	payload := GotifyPayload{
		Message:  message,
		Priority: priority,
		Title:    title,
		Extras:   map[string]interface{}{},
	}

	payload.HandleExtras(g.Extras, defaults.Extras, serviceURL)

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, gotifyURL, bytes.NewReader(payloadData))
	req.Header.Add("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	if err != nil {
		msg := fmt.Sprintf("%s (%s), Gotify\n%s", svc.ID, monitorID, err)
		jLog.Verbose(msg, true)
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// If verbose or above, print the error every time
		msg := fmt.Sprintf("%s (%s), Slack\n%s", svc.ID, monitorID, err)
		jLog.Verbose(msg, true)
		return err
	}
	defer resp.Body.Close()

	// SUCCESS (2XX)
	if strconv.Itoa(resp.StatusCode)[:1] == "2" {
		msg := fmt.Sprintf("%s (%s), Gotify message sent", svc.ID, monitorID)
		jLog.Info(msg, true)
		return nil
	}

	// FAIL
	return fmt.Errorf("%s (%s), Gotify message failed to send.\n%s", svc.ID, monitorID, err)
}
