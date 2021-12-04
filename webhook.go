package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

// WebHookSlice is an array of WebHook.
type WebHookSlice []WebHook

// WebHook is a WebHook to send.
type WebHook struct {
	Type              string `yaml:"type"`                          // "github"/"url"
	URL               string `yaml:"url"`                           // "https://example.com"
	Secret            string `yaml:"secret,omitempty"`              // "SECRET"
	DesiredStatusCode int    `yaml:"desired_status_code,omitempty"` // e.g. 202
	Delay             string `yaml:"delay,omitempty"`               // The delay before sending the WebHook.
	MaxTries          uint   `yaml:"maxtries,omitempty"`            // Number of times to attempt sending the WebHook if the desired status code is not received.
	SilentFails       string `yaml:"silent_fails,omitempty"`        // Whether to notify if this WebHook fails MaxTries times.
}

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of a dict.
//
// e.g.    WebHook: { url: "example.com" }
//
// becomes WebHook: [ { url: "example.com" } ]
func (w *WebHookSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []WebHook
	err := unmarshal(&multi)
	if err != nil {
		var single WebHook
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*w = []WebHook{single}
	} else {
		*w = multi
	}
	return nil
}

// setDefaults sets undefined variables to their default.
func (w *WebHookSlice) setDefaults(monitorID string, defaults Defaults) {
	for index := range *w {
		(*w)[index].setDefaults(defaults)
	}
	(*w).checkValues(monitorID)
}

// setDefaults sets undefined variables to their default.
func (w *WebHook) setDefaults(defaults Defaults) {
	// DesiredStatusCode
	w.DesiredStatusCode = valueOrValueInt(w.DesiredStatusCode, defaults.WebHook.DesiredStatusCode)

	// Delay
	w.Delay = valueOrValueString(w.Delay, defaults.WebHook.Delay)

	// MaxTries
	w.MaxTries = valueOrValueUInt(w.MaxTries, defaults.WebHook.MaxTries)

	// SilentFails
	w.SilentFails = valueOrValueString(w.SilentFails, defaults.WebHook.SilentFails)
	w.SilentFails = stringBool(w.SilentFails, "", "", false)
}

// checkValues will check the variables for all of this Monitor's WebHook recipients.
func (w *WebHookSlice) checkValues(monitorID string) {
	for index := range *w {
		(*w)[index].checkValues(monitorID, index, len(*w) == 1)
	}
}

// checkValues will check that the variables are valid for this WebHook recipient.
func (w *WebHook) checkValues(monitorID string, index int, loneService bool) {
	target := monitorID + ".webhook"
	if !loneService {
		target = fmt.Sprintf("%s[%d]", monitorID, index)
	}

	// Delay
	if w.Delay != "" {
		// Default to seconds when an integer is provided
		if _, err := strconv.Atoi(w.Delay); err == nil {
			w.Delay += "s"
		}
		if _, err := time.ParseDuration(w.Delay); err != nil {
			fmt.Printf("ERROR: %s.delay (%s) is invalid (Use 'AhBmCs' duration format)", target, w.Delay)
			os.Exit(1)
		}
	}
}

// WebHookGitHub is the WebHook payload to emulate GitHub.
type WebHookGitHub struct {
	Ref    string `json:"ref"`    // "refs/heads/master"
	Before string `json:"before"` // "randAlphaNumericLower(40)"
	After  string `json:"after"`  // "randAlphaNumericLower(40)"
}

// randString will make a random string of length n with alphabet.
func randString(n int, alphabet string) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

const numeric = "0123456789"

// randNumeric will return a random numeric string of length n.
func randNumeric(n int) string {
	return randString(n, numeric)
}

const alphanumericLower = "abcdefghijklmnopqrstuvwxyz0123456789"

// randAlphaNumericLower will return a random alphanumeric (lowercase) string of length n.
func randAlphaNumericLower(n int) string {
	return randString(n, alphanumericLower)
}

// send will send every WebHook in this WebHookSlice with a delay between each webhook.
func (w *WebHookSlice) send(monitorID string, serviceID string, slacks SlackSlice) {
	for index := range *w {
		go func() {
			index := index                    // Create new instance for the goroutine.
			triesLeft := (*w)[index].MaxTries // Number of times to send WebHook (until w.DesiredStatusCode received).

			// Delay sending the Slack message by the defined interval.
			sleepTime, _ := time.ParseDuration((*w)[index].Delay)
			msg := fmt.Sprintf("%s (%s), Sleeping for %s before sending the WebHook", serviceID, monitorID, (*w)[index].Delay)
			logInfo(*logLevel, msg, (sleepTime != 0))
			time.Sleep(sleepTime)

			for {
				err := (*w)[index].send(monitorID, serviceID)

				// SUCCESS!
				if err == nil {
					break
				}

				// FAIL!
				triesLeft--
				// Give up after MaxTries.
				if triesLeft == 0 {
					// If not verbose or above (above, this would already have been printed).
					msg := fmt.Sprintf("%s (%s), %s", serviceID, monitorID, err)
					logError(msg, (*logLevel < 3))
					message := fmt.Sprintf("%s, Failed %d times to send a WebHook to %s", monitorID, (*w)[index].MaxTries, (*w)[index].URL)
					if (*w)[index].SilentFails == "n" {
						svc := Service{
							ID: serviceID,
						}
						slacks.send(monitorID, &svc, message)
					}
					log.Printf("ERROR: %s (%s), %s", serviceID, monitorID, message)
					break
				}
				// Space out retries.
				time.Sleep(10 * time.Second)
			}
		}()
		// Space out WebHooks.
		time.Sleep(3 * time.Second)
	}
}

// send will send a WebHook to the WebHook URL with the body SHA1 and SHA256 encrypted with WebHook.Secret.
// It also simulates other GitHub headers and returns when an error is encountered.
func (w *WebHook) send(monitorID string, serviceID string) error {
	// GitHub style payload.
	payload, err := json.Marshal(WebHookGitHub{Ref: "refs/heads/master", Before: randAlphaNumericLower(40), After: randAlphaNumericLower(40)})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// GitHub style headers.
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Hook-ID", randNumeric(9))
	req.Header.Set("X-GitHub-Delivery", fmt.Sprintf("%s-%s-%s-%s-%s", randAlphaNumericLower(8), randAlphaNumericLower(4), randAlphaNumericLower(4), randAlphaNumericLower(4), randAlphaNumericLower(12)))
	req.Header.Set("X-GitHub-Hook-Installation-Target-ID", randNumeric(9))
	req.Header.Set("X-GitHub-Hook-Installation-Target-Type", "repository")

	// X-Hub-Signature-256.
	hash := hmac.New(sha256.New, []byte(w.Secret))
	hash.Write(payload)
	req.Header.Set("X-Hub-Signature-256", fmt.Sprintf("sha256=%s", hex.EncodeToString(hash.Sum(nil))))

	// X-Hub-Signature.
	hash = hmac.New(sha1.New, []byte(w.Secret))
	hash.Write(payload)
	req.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", hex.EncodeToString(hash.Sum(nil))))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// If verbose or above, print the error every time
		msg := fmt.Sprintf("%s (%s), WebHook:\n%s", serviceID, monitorID, err)
		logError(msg, (*logLevel > 2))
		return err
	}
	defer resp.Body.Close()

	// SUCCESS
	if resp.StatusCode == w.DesiredStatusCode || (w.DesiredStatusCode == 0 && (strconv.Itoa(resp.StatusCode)[:1] == "2")) {
		msg := fmt.Sprintf("%s (%s), (%d) WebHook received", serviceID, monitorID, resp.StatusCode)
		logInfo(*logLevel, msg, true)
		return nil
	}

	// FAIL
	body, _ := ioutil.ReadAll(resp.Body)

	// Pretty desiredStatusCode.
	desiredStatusCode := strconv.Itoa(w.DesiredStatusCode)
	if desiredStatusCode == "0" {
		desiredStatusCode = "2XX"
	}

	// If verbose or above, print the error every time
	msg := fmt.Sprintf("%s (%s), WebHook didn't %s:\n%s\n%s", serviceID, monitorID, desiredStatusCode, resp.Status, body)
	logError(msg, (*logLevel > 2))
	return fmt.Errorf("%s, %s", resp.Status, body)
}
