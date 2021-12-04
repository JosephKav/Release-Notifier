package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
)

// ServiceSlice is an array of Service.
type ServiceSlice []Service

// Service is a source to be serviceed and provides everything needed to extract
// the latest version from the URL provided.
type Service struct {
	ID                    string          `yaml:"id"`
	Type                  string          `yaml:"type"`                   // "github"/"URL"
	URL                   string          `yaml:"url"`                    // type:URL - "https://example.com", type:github - "owner/repo" or "https://github.com/owner/repo".
	URLCommands           URLCommandSlice `yaml:"url_commands"`           // Commands to filter the release from the URL request.
	Interval              string          `yaml:"interval"`               // AhBmCs = Sleep A hours, B minutes and C seconds between queries.
	ProgressiveVersioning string          `yaml:"progressive_versioning"` // default - true  = Version has to be greater than the previous to trigger Slack(s)/WebHook(s).
	RegexContent          string          `yaml:"regex_content"`          // "abc-[a-z]+-${version}_amd64.deb" This regex must exist in the body of the URL to trigger new version actions.
	RegexVersion          string          `yaml:"regex_version"`          // "v*[0-9.]+" The version found must match this release to trigger new version actions.
	SkipSlack             bool            `yaml:"skip_slack"`             // default - false = Don't skip Slack messages for new releases.
	SkipWebHook           bool            `yaml:"skip_webhook"`           // default - false = Don't skip WebHooks for new releases.
	IgnoreMiss            string          `yaml:"ignore_misses"`          // Ignore URLCommands that fail (e.g. split on text that doesn't exist)
	AccessToken           string          `yaml:"access_token"`           // GitHub access token to use.
	AllowInvalidCerts     string          `yaml:"allow_invalid"`          // default - false = Disallows invalid HTTPS certificates.
	Slack                 Slack           `yaml:"slack"`                  // Override Slack message vars.
	status                status          ``                              // Track the Status of this source (version and regex misses).
}

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of dict's.
//
// e.g.    ServiceSlice: { URL: "example.com" }
//
// becomes ServiceSlice: [ { URL: "example.com" } ]
func (s *ServiceSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []Service
	err := unmarshal(&multi)
	if err != nil {
		var single Service
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*s = []Service{single}
	} else {
		*s = multi
	}
	return nil
}

// URLCommandSlice is an array of URLCommand to be used to filter version from the URL Content.
type URLCommandSlice []URLCommand

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of a dict.
//
// e.g.    URLCommandSlice: { type: "split" }
//
// becomes URLCommandSlice: [ { type: "split" } ]
func (c *URLCommandSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []URLCommand
	err := unmarshal(&multi)
	if err != nil {
		var single URLCommand
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*c = []URLCommand{single}
	} else {
		*c = multi
	}
	return nil
}

// print will print the URLCommand's in the URLCommandSlice
func (c *URLCommandSlice) print(prefix string) {
	noCmds := ""
	if len(*c) == 0 {
		noCmds = " []"
	}
	fmt.Printf("%surl_commands:%s\n", prefix, noCmds)
	for _, command := range *c {
		command.print(prefix)
	}
}

// URLCommand is a command to be ran to filter version from the URL body.
type URLCommand struct {
	Type       string `yaml:"type"`          // "regex"/"regex_submatch"/"replace"/"split"
	Regex      string `yaml:"regex"`         // regexp.MustCompile(Regex)
	Index      int    `yaml:"index"`         // re.FindAllString(URL_content, -1)[Index]  /  strings.Split("text")[Index]
	Old        string `yaml:"old"`           // strings.ReplaceAll(tgtString, "Old", "New")
	New        string `yaml:"new"`           // strings.ReplaceAll(tgtString, "Old", "New")
	Text       string `yaml:"text"`          // strings.Split(tgtString, "Text")
	IgnoreMiss string `yaml:"ignore_misses"` // Ignore this command failing (e.g. split on text that doesn't exist)
}

// print will print the URLCommand
func (c *URLCommand) print(prefix string) {
	fmt.Printf("%s  - type: %s\n", prefix, c.Type)
	switch c.Type {
	case "regex":
		fmt.Printf("%s    regex: '%s'\n", prefix, c.Regex)
		fmt.Printf("%s    ignore_misses: %s\n", prefix, c.IgnoreMiss)
	case "regex_submatch":
		fmt.Printf("%s    regex: '%s'\n", prefix, c.Regex)
		fmt.Printf("%s    index: %d\n", prefix, c.Index)
		fmt.Printf("%s    ignore_misses: %s\n", prefix, c.IgnoreMiss)
	case "replace":
		fmt.Printf("%s    new: '%s'\n", prefix, c.New)
		fmt.Printf("%s    old: '%s'\n", prefix, c.Old)
	case "split":
		fmt.Printf("%s    text: '%s'\n", prefix, c.Text)
		fmt.Printf("%s    index: %d\n", prefix, c.Index)
		fmt.Printf("%s    ignore_misses: %s\n", prefix, c.IgnoreMiss)
	}
}

// setDefaults sets undefined variables to their default.
func (c *URLCommandSlice) run(monitorID string, service *Service, text string) (string, error) {
	var err error
	for commandIndex := range *c {
		text, err = (*c)[commandIndex].run(monitorID, service, text)
		if err != nil {
			return text, err
		}
	}
	return text, nil
}

func (c *URLCommand) run(monitorID string, service *Service, text string) (string, error) {
	// Iterate through the commands to filter the text.
	textBak := text
	msg := fmt.Sprintf("Looking through %s", text)
	logDebug(*logLevel, msg, true)

	var err error = nil

	switch c.Type {
	case "split":
		text, err = c.split(monitorID, *service, text)
	case "replace":
		text = strings.ReplaceAll(text, c.Old, c.New)
	case "regex", "regex_submatch":
		text, err = c.regex(monitorID, *service, text)
	}
	if err != nil {
		return textBak, nil
	}

	msg = fmt.Sprintf("%s (%s), Resolved to %s", service.ID, monitorID, text)
	logDebug(*logLevel, msg, true)
	return text, nil
}

func (c *URLCommand) regex(monitorID string, service Service, text string) (string, error) {
	re := regexp.MustCompile(c.Regex)

	var texts []string
	switch c.Type {
	case "regex":
		texts = re.FindAllString(text, -1)
	case "regex_submatch":
		if c.Index < 0 {
			msg := fmt.Sprintf("%s (%s), %s (%s) shouldn't use negative indices as the array is always made up from the first match.", service.ID, monitorID, c.Type, c.Regex)
			logWarn(*logLevel, msg, true)
		}
		texts = re.FindStringSubmatch(text)
	}

	if len(texts) == 0 {
		msg := fmt.Sprintf("%s (%s), %s (%s) didn't return any matches", service.ID, monitorID, c.Type, c.Regex)
		if getAtIndex(service.status.serviceMisses, 2) == "0" {
			logWarn(*logLevel, msg, true)
			service.status.serviceMisses = replaceAtIndex(service.status.serviceMisses, '1', 2)
		}
		// Stop if miss.
		if c.IgnoreMiss == "n" {
			return text, errors.New(msg)
		}
		// Ignore Misses.
		return text, nil
	}

	index := c.Index
	// Handle negative indices.
	if c.Index < 0 {
		index = len(texts) + c.Index
	}

	if (len(texts) - index) < 1 {
		msg := fmt.Sprintf("%s (%s), %s (%s) returned %d elements but the index wants element number %d", service.ID, monitorID, c.Type, c.Regex, len(texts), (index + 1))
		if getAtIndex(service.status.serviceMisses, 3) == "0" {
			logWarn(*logLevel, msg, true)
			service.status.serviceMisses = replaceAtIndex(service.status.serviceMisses, '1', 3)
		}
		// Stop if miss.
		if c.IgnoreMiss == "n" {
			return text, errors.New(msg)
		}
		// Ignore Misses.
		return text, nil
	}

	return texts[index], nil
}

func (c *URLCommand) split(monitorID string, service Service, text string) (string, error) {
	texts := strings.Split(text, c.Text)

	if len(texts) == 1 {
		msg := fmt.Sprintf("%s (%s), %s didn't find any '%s' to split on", service.ID, monitorID, c.Type, c.Text)
		if getAtIndex(service.status.serviceMisses, 0) == "0" {
			logWarn(*logLevel, msg, true)
			service.status.serviceMisses = replaceAtIndex(service.status.serviceMisses, '1', 0)
		}
		// Stop if miss.
		if c.IgnoreMiss == "n" {
			return text, errors.New(msg)
		}
		// Ignore Misses.
		return text, nil
	}

	index := c.Index
	// Handle negative indices.
	if index < 0 {
		index = len(texts) + index
	}

	if (len(texts) - index) < 1 {
		msg := fmt.Sprintf("%s (%s), %s (%s) returned %d elements but the index wants element number %d", service.ID, monitorID, c.Type, c.Text, len(texts), (index + 1))
		if getAtIndex(service.status.serviceMisses, 1) == "0" {
			logWarn(*logLevel, msg, true)
			service.status.serviceMisses = replaceAtIndex(service.status.serviceMisses, '1', 1)
		}
		// Stop if miss.
		if c.IgnoreMiss == "n" {
			return text, errors.New(msg)
		}
		// Ignore Misses.
		return text, nil
	}

	return texts[index], nil
}

// checkValues will check the variables for the URLCommand's in the URLCommandSlice.
func (c *URLCommandSlice) checkValues(monitorID string, serviceID string) {
	for index := range *c {
		(*c)[index].checkValues(monitorID, serviceID)
	}
}

// checkValues will check the variables for the URLCommand.
func (c *URLCommand) checkValues(monitorID string, serviceID string) {
	switch c.Type {
	case "split", "replace", "regex", "regex_submatch":
	default:
		msg := fmt.Sprintf("%s (%s), %s is an unknown type for url_commands", serviceID, monitorID, c.Type)
		logFatal(msg, true)
	}
}

// checkValues will check the variables for the Service's in the ServiceSlice.
func (s *ServiceSlice) checkValues(monitorID string) {
	for index := range *s {
		(*s)[index].checkValues(monitorID, index, len(*s) == 1)
		(*s)[index].URLCommands.checkValues(monitorID, (*s)[index].ID)
	}

}

// checkValues will check the variables for the Service.
func (s *Service) checkValues(monitorID string, index int, loneService bool) {
	target := monitorID
	if !loneService {
		target = fmt.Sprintf("%s[%d]", monitorID, index)
	}

	// Interval
	if s.Interval != "" {
		// Default to seconds when an integer is provided
		if _, err := strconv.Atoi(s.Interval); err == nil {
			s.Interval += "s"
		}
		if _, err := time.ParseDuration(s.Interval); err != nil {
			msg := fmt.Sprintf("%s.interval (%s) is invalid (Use 'AhBmCs' duration format)", target, s.Interval)
			logFatal(msg, true)
		}
	}

	// Slack - Delay
	if s.Slack.Delay != "" {
		if _, err := time.ParseDuration(s.Slack.Delay); err != nil {
			msg := fmt.Sprintf("%s.slack.delay (%s) is invalid (Use 'AhBmCs' duration format)", target, s.Slack.Delay)
			logFatal(msg, true)
		}
	}
}

// status is the current state of the Service element (version and regex misses).
type status struct {
	version            string // Latest version found from query().
	regexMissesContent uint   // Counter for the number of regex misses on URL content.
	regexMissesVersion uint   // Counter for the number of regex misses on version.
	serviceMisses      string // "1000" 1 = miss, 0 = no miss for split etc.
}

// init initialises the status vars when more than the default value is needed.
func (s *status) init() {
	s.serviceMisses = "0000"
}

// setDefaults sets undefined variables to their default.
func (s *ServiceSlice) setDefaults(monitorID string, defaults Defaults) {
	for index := range *s {
		(*s)[index].setDefaults(defaults)
	}
	(*s).checkValues(monitorID)
}

// setDefaults sets undefined variables to their default.
func (s *Service) setDefaults(defaults Defaults) {
	// Default GitHub Access Token.
	s.AccessToken = valueOrValueString(s.AccessToken, defaults.Service.AccessToken)

	// Default allowance/rejection of invalid certs.
	s.AllowInvalidCerts = valueOrValueString(s.AllowInvalidCerts, defaults.Service.AllowInvalidCerts)
	s.AllowInvalidCerts = stringBool(s.AllowInvalidCerts, "", "", false)

	// Default progressive versioning (versions have to be successive to notify)
	s.ProgressiveVersioning = valueOrValueString(s.ProgressiveVersioning, defaults.Service.ProgressiveVersioning)
	s.ProgressiveVersioning = stringBool(s.ProgressiveVersioning, "", "", true)

	// Default ID if undefined/blank.
	if s.ID == "" {
		// Set s.ID to github "owner/repo".
		if s.Type == "github" {
			// Preserve owner/repo strings.
			if strings.Count(s.URL, "/") == 1 {
				s.ID = s.URL
			} else {
				// Filter "owner/repo" out of "(https://)github.com/owner/repo...".
				splitURL := strings.Split(s.URL, ".com/")
				splitURL = strings.SplitN(splitURL[1], "/", 2)
				owner := splitURL[0]
				repo := strings.Split(splitURL[1], "/")[0]
				s.ID = fmt.Sprintf("%s/%s", owner, repo)
			}
			// Set s.ID to website (e.g. https://test.com/releases = test)
		} else if s.Type == "url" {
			// Filter owner/repo out if we're using a github URL rather than the API (type=github)
			if strings.Contains(s.URL, "github.com/") {
				splitURL := strings.Split(s.URL, ".com/")
				splitURL = strings.SplitN(splitURL[1], "/", 2)
				owner := splitURL[0]
				repo := strings.Split(splitURL[1], "/")[0]
				s.ID = fmt.Sprintf("%s/%s", owner, repo)
			} else {
				s.ID = s.URL
				// if s.URL starts with http(s)://
				if strings.Contains(s.ID[:8], "://") {
					s.ID = strings.Split(s.ID, "://")[1]
				}
				s.ID = strings.Split(s.ID, "/")[0]
				s.ID = strings.Split(s.ID, ".")[0]
			}
		}
	}

	// Default Interval.
	s.Interval = valueOrValueString(s.Interval, defaults.Service.Interval)

	// Default Type.
	if s.Type == "" {
		if strings.Count(s.URL, "/") == 1 {
			s.Type = "github"
		} else {
			s.Type = "URL"
		}
	}

	// GitHub - Convert to API URL.
	if s.Type == "github" {
		// If it's "owner/repo" rather than a full path.
		if strings.Count(s.URL, "/") == 1 {
			s.URL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", s.URL)

			// Convert "https://github.com/owner/repo" to API path.
			// Don't modify URLs that already point to the API.
		} else if !strings.Contains(s.URL, "api.github") {
			s.URL = strings.Split(s.URL, "github.")[1]
			// split "com/owner/repo" on "/" and keep two substrings, "com" and "owner/repo"
			s.URL = strings.SplitN(s.URL, "/", 2)[1]
			s.URL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", s.URL)
		}
	}

	s.IgnoreMiss = valueOrValueString(s.IgnoreMiss, defaults.Service.IgnoreMiss)
	s.IgnoreMiss = stringBool(s.IgnoreMiss, "", "", false)

	s.URLCommands.setDefaults(defaults, s)
}

// setDefaults sets undefined variables to their default.
func (c *URLCommandSlice) setDefaults(defaults Defaults, service *Service) {
	for index := range *c {
		(*c)[index].setDefaults(defaults, service)
	}
}

// setDefaults sets undefined variables to their default.
func (c *URLCommand) setDefaults(defaults Defaults, service *Service) {
	// Default IgnoreMiss.
	c.IgnoreMiss = valueOrValueString(c.IgnoreMiss, defaults.Service.IgnoreMiss)
	c.IgnoreMiss = stringBool(c.IgnoreMiss, "", "", false)
}

// setVersion sets Service.Version to v.
func (s *Service) setVersion(v string) {
	s.status.version = v
}

// regexCheckContent returns whether there is a regex match of re on text.
func regexCheck(re string, text string) bool {
	regex := regexp.MustCompile(re)
	// Return whether there's a regex match.
	return regex.MatchString(text)
}

// regexCheckContent returns the result of a regex match of re on text
// after replacing "${version}" with the version string and "${version_no_v}"
// with the version string minus any "v"s in it.
func regexCheckContent(re string, text string, version string) bool {
	re = strings.ReplaceAll(re, "${version}", version)
	re = strings.ReplaceAll(re, "${version_no_v}", strings.ReplaceAll(version, "v", ""))
	return regexCheck(re, text)
}

// replaceAtIndex replaces the character at index of str with replacement
func replaceAtIndex(str string, replacement rune, index int) string {
	return str[:index] + string(replacement) + str[index+1:]
}

// getAtIndex returns the character at index of str.
func getAtIndex(str string, index int) string {
	return str[index : index+1]
}

// query queries the Service source, updating Service.Version
// and returning true if it has changed (is a new release),
// otherwise returns false.
//
// index = index of this Service in the parent Monitor
// monitorID = ID of the parent Monitor
func (s *Service) query(index int, monitorID string) bool {
	customTransport := &http.Transport{}
	// HTTPS insecure skip verify.
	if s.AllowInvalidCerts == "y" {
		customTransport = http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		msg := fmt.Sprintf("%s, %s", s.ID, err)
		logError(msg, true)
		return false
	}

	if s.AccessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", s.AccessToken))
	}

	client := &http.Client{Transport: customTransport}
	resp, err := client.Do(req)

	if err != nil {
		// Don't crash on invalid certs.
		if strings.Contains(err.Error(), "x509") {
			msg := fmt.Sprintf("x509 for %s (%s) (Cert invalid)", s.ID, monitorID)
			logWarn(*logLevel, msg, true)
			return false
		}
		msg := fmt.Sprintf("%s (%s), %s", s.ID, monitorID, err)
		logError(msg, true)
		return false
	}

	// Read the response body.
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("%s (%s), %s", s.ID, monitorID, err)
		logError(msg, true)
		return false
	}
	// Convert the body to string.
	body := string(rawBody)
	version := body

	// GitHub service.
	if s.Type == "github" {
		// Check for rate limit.
		if len(body) < 500 {
			if !strings.Contains(body, `"tag_name"`) {
				msg := "GitHub Access Token is invalid!"
				logFatal(msg, strings.Contains(body, "Bad credentials"))

				msg = fmt.Sprintf("tag_name not found for %s (%s) at %s\n%s", s.ID, monitorID, s.URL, body)
				logError(msg, true)
				return false
			}
			if strings.Contains(body, "rate limit") {
				msg := fmt.Sprintf("Rate limit reached on %s (%s)", s.ID, monitorID)
				logWarn(*logLevel, msg, true)
				return false
			}
		}
		version = strings.Split(body, `"tag_name"`)[1]
		version = strings.Split(version, ",")[0]
		version = strings.Split(version, `"`)[1]
		// Raw URL Service.
	}

	// Iterate through the commands to filter out the version.
	version, err = s.URLCommands.run(monitorID, s, version)
	// If URLCommands failed, return
	if err != nil {
		return false
	}

	// If this version is different (new).
	if version != s.status.version {
		// Check for a progressive change in version.
		if s.ProgressiveVersioning == "y" && s.status.version != "" {
			failedSemanticVersioning := false
			oldVersion, err := semver.NewVersion(s.status.version)
			if err != nil {
				msg := fmt.Sprintf("%s (%s), failed converting '%s' to a semantic version", s.ID, monitorID, s.status.version)
				logError(msg, true)
				failedSemanticVersioning = true
			}
			newVersion, err := semver.NewVersion(version)
			if err != nil {
				msg := fmt.Sprintf("%s (%s), failed converting '%s' to a semantic version", s.ID, monitorID, version)
				logError(msg, true)
				failedSemanticVersioning = true
			}

			// e.g.
			// newVersion = 1.2.9
			// oldVersion = 1.2.10
			// return false (don't notify anything. Stay on oldVersion)
			if !failedSemanticVersioning && newVersion.LessThan(*oldVersion) {
				return false
			}
		}

		// Check for a regex match in the body if one is desired.
		if s.RegexContent != "" {
			regexMatch := regexCheckContent(s.RegexContent, body, version)
			if !regexMatch {
				msg := fmt.Sprintf("%s (%s), Regex not matched on content for version %s", s.ID, monitorID, version)
				s.status.regexMissesContent++
				logVerbose(*logLevel, msg, s.status.regexMissesContent == 1)
				return false
			}
		}
		// Check that the version grabbed satisfies the specified regex (if there is any).
		if s.RegexVersion != "" {
			regexMatch := regexCheck(s.RegexVersion, version)
			if !regexMatch {
				msg := fmt.Sprintf("%s (%s), Regex not matched on version %s", s.ID, monitorID, version)
				s.status.regexMissesVersion++
				logVerbose(*logLevel, msg, s.status.regexMissesVersion == 1)
				return false
			}
		}

		// Found new version, so reset regex misses.
		s.status.regexMissesContent = 0
		s.status.regexMissesVersion = 0

		// First version found.
		if s.status.version == "" {
			if s.ProgressiveVersioning == "y" {
				if _, err := semver.NewVersion(version); err != nil {
					msg := fmt.Sprintf("%s (%s), failed converting '%s' to a semantic version. If all versions are in this style, consider adding url_commands to get the version into the style of '1.2.3a' (https://semver.org/), or disabling progressive versioning (globally with defaults.service.progressive_versioning or just for this service with the progressive_versioning var)", s.ID, monitorID, version)
					logFatal(msg, true)
				}
			}

			s.setVersion(version)
			msg := fmt.Sprintf("%s (%s), Starting Release - %s", s.ID, monitorID, version)
			logInfo(*logLevel, msg, true)
			// Don't notify on first version.
			return false
		}

		// New version found.
		s.setVersion(version)
		msg := fmt.Sprintf("%s (%s), New Release - %s", s.ID, monitorID, version)
		logInfo(*logLevel, msg, true)
		return true
	}

	// No version change.
	return false
}
