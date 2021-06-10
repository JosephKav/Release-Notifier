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

	"time"
)

// WebHookSlice is an array of WebHook's.
type WebHookSlice []WebHook

// WebHook is a WebHook to send.
type WebHook struct {
	Type              string `yaml:"type"`                // "github"/"url"
	URL               string `yaml:"url"`                 // "https://example.com"
	Secret            string `yaml:"secret"`              // "SECRET"
	DesiredStatusCode int    `yaml:"desired_status_code"` // e.g. 202
	MaxTries          int    `yaml:"maxtries"`            // Number of times to attempt sending the WebHook if the desired status code is not received.
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

// setDefaults calls setDefaults on each WebHook to set the defaults for undefined values.
func (w *WebHookSlice) setDefaults(defaults Defaults) {
	for index := range *w {
		(*w)[index].setDefaults(defaults)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (w *WebHook) setDefaults(defaults Defaults) {
	if w.DesiredStatusCode == 0 {
		w.DesiredStatusCode = defaults.WebHook.DesiredStatusCode
	}

	if w.MaxTries == 0 {
		w.MaxTries = defaults.WebHook.MaxTries
	}
}

// WebHookGitHub is the WebHook payload to emulate GitHub.
type WebHookGitHub struct {
	Ref    string `json:"ref"`    // "refs/heads/master"
	Before string `json:"before"` // "randAlphaNumericLower(40)"
	After  string `json:"after"`  // "randAlphaNumericLower(40)"
}

const numeric = "0123456789"

// randNumeric will return a random numeric string of length n.
func randNumeric(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = numeric[rand.Intn(len(numeric))]
	}
	return string(b)
}

const alphanumericLower = "abcdefghijklmnopqrstuvwxyz0123456789"

// randAlphaNumericLower will return a random alphanumeric (lowercase) string of length n.
func randAlphaNumericLower(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = alphanumericLower[rand.Intn(len(alphanumericLower))]
	}
	return string(b)
}

// send will send every WebHook in this WebHookSlice with a delay between each webhook.
func (w *WebHookSlice) send(serviceID string) {
	for index := range *w {
		go func() {
			index := index                    // Create new instance for the goroutine.
			triesLeft := (*w)[index].MaxTries // Number of times to send WebHook (until w.DesiredStatusCode received).
			for {
				err := (*w)[index].send(serviceID)

				// SUCCESS!
				if err == nil {
					break
				}

				// FAIL!
				triesLeft--
				// Give up after MaxTries.
				if triesLeft == 0 {
					// If not verbose (this would already have been printed in verbose).
					if !*verbose {
						log.Printf("ERROR: %s", err)
					}
					log.Printf("ERROR: %s, Failed %d times to send webhook to %s", serviceID, (*w)[index].MaxTries, (*w)[index].URL)
					break
				}
				// Space out retries.
				time.Sleep(10 * time.Second)
			}
		}()
		// Space out Slack messages.
		time.Sleep(3 * time.Second)
	}
}

// send will send a WebHook to the WebHook's URL with the body sha1 and sha256 encrypted with WebHook.Secret.
// It also simulates other GitHub headers and returns when an error is encountered.
func (w *WebHook) send(serviceID string) error {
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
		if *verbose {
			log.Printf("ERROR: WebHook %s", err)
		}
		return err
	}
	defer resp.Body.Close()

	// SUCCESS!
	if resp.StatusCode == w.DesiredStatusCode || w.DesiredStatusCode == 0 {
		log.Printf("INFO: %s, WebHook received (%d)", serviceID, resp.StatusCode)
		return nil
	}

	// FAIL!
	body, _ := ioutil.ReadAll(resp.Body)
	if *verbose {
		log.Printf("ERROR: Request didn't respond with %d. Got a %s, %s", w.DesiredStatusCode, resp.Status, body)
	}
	return fmt.Errorf("request didn't respond with %d: Got a %s, %s", w.DesiredStatusCode, resp.Status, body)
}
