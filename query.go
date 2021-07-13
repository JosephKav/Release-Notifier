package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// MonitorSlice is an array of Monitor.
type MonitorSlice []Monitor

// Monitor is a source to be monitored and provides everything needed to extract
// the latest version from the URL provided.
type Monitor struct {
	ID                string          `yaml:"id"`
	Type              string          `yaml:"type"`          // "github"/"URL"
	URL               string          `yaml:"url"`           // type:URL - "https://example.com", type:github - "owner/repo" or "https://github.com/owner/repo".
	URLCommands       URLCommandSlice `yaml:"url_commands"`  // Commands to filter the release from the URL request.
	RegexContent      string          `yaml:"regex_content"` // "abc-[a-z]+-${version}_amd64.deb" This regex must exist in the body of the URL to trigger new version actions.
	RegexVersion      string          `yaml:"regex_version"` // "v*[0-9.]+" The version found must match this release to trigger new version actions.
	AllowInvalidCerts string          `yaml:"allow_invalid"` // default - false = Disallows invalid HTTPS certificates.
	AccessToken       string          `yaml:"access_token"`  // GitHub access token to use.
	SkipSlack         bool            `yaml:"skip_slack"`    // default - false = Don't skip Slack messages for new releases.
	SkipWebHook       bool            `yaml:"skip_webhook"`  // default - false = Don't skip WebHooks for new releases.
	Interval          int             `yaml:"interval"`      // 600 = Sleep 600 seconds between queries.
	status            status          ``                     // Track the Status of this source (version and regex misses).
}

// UnmarshalYAML allows handling of a dict as well as a list of dicts.
//
// It will convert a dict to a list of a dict.
//
// e.g.    MonitorSlice: { URL: "example.com" }
//
// becomes MonitorSlice: [ { URL: "example.com" } ]
func (m *MonitorSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []Monitor
	err := unmarshal(&multi)
	if err != nil {
		var single Monitor
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*m = []Monitor{single}
	} else {
		*m = multi
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

// status is the current state of the Monitor element (version and regex misses).
type status struct {
	version            string // Latest version found from query().
	regexMissesContent int    // Counter for the number of regex misses on URL content.
	regexMissesVersion int    // Counter for the number of regex misses on version.
	monitorMisses      string // "1000" 1 = miss, 0 = no miss for split etc.
}

// setDefaults sets the defaults for undefined Monitor values using defaults.
func (s *status) init() {
	s.monitorMisses = "0000"
}

// setDefaults sets the defaults for undefined Monitor values using defaults.
func (m *MonitorSlice) setDefaults(defaults Defaults) {
	for index := range *m {
		(*m)[index].setDefaults(defaults)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (m *Monitor) setDefaults(defaults Defaults) {
	// Default GitHub Access Token
	if m.AccessToken == "" {
		m.AccessToken = defaults.Monitor.AccessToken
	}

	// Default allowance/rejection of invalid certs
	if m.AllowInvalidCerts == "" {
		m.AllowInvalidCerts = defaults.Monitor.AllowInvalidCerts
	} else if strings.ToLower(m.AllowInvalidCerts) == "true" || strings.ToLower(m.AllowInvalidCerts) == "yes" {
		m.AllowInvalidCerts = "y"
	} else {
		m.AllowInvalidCerts = "n"
	}

	// Default ID if undefined/blank.
	if m.ID == "" {
		// Set m.ID to github "owner/repo".
		if m.Type == "github" {
			// Preserve owner/repo strings.
			if strings.Count(m.URL, "/") == 1 {
				m.ID = m.URL
			} else {
				// Filter "owner/repo" out of "(https://)github.com/owner/repo...".
				splitURL := strings.Split(m.URL, ".com/")
				splitURL = strings.SplitN(splitURL[1], "/", 2)
				owner := splitURL[0]
				repo := strings.Split(splitURL[1], "/")[0]
				m.ID = fmt.Sprintf("%s/%s", owner, repo)
			}
			// Set m.ID to website (e.g. https://test.com/releases = test)
		} else if m.Type == "url" {
			// Filter owner/repo out if we're using a github URL rather than the API (type=github)
			if strings.Contains(m.URL, "github.com/") {
				splitURL := strings.Split(m.URL, ".com/")
				splitURL = strings.SplitN(splitURL[1], "/", 2)
				owner := splitURL[0]
				repo := strings.Split(splitURL[1], "/")[0]
				m.ID = fmt.Sprintf("%s/%s", owner, repo)
			} else {
				m.ID = m.URL
				// if m.URL starts with http(s)://
				if strings.Contains(m.ID[:8], "://") {
					m.ID = strings.Split(m.ID, "://")[1]
				}
				m.ID = strings.Split(m.ID, "/")[0]
				m.ID = strings.Split(m.ID, ".")[0]
			}
		}
	}

	// Default Interval.
	if m.Interval == 0 {
		m.Interval = defaults.Monitor.Interval
	}

	// Default Type.
	if m.Type == "" {
		if strings.Count(m.URL, "/") == 1 {
			m.Type = "github"
		} else {
			m.Type = "URL"
		}
	}

	// GitHub - Convert to API URL
	if m.Type == "github" {
		// If it's "owner/repo" rather than a full path.
		if strings.Count(m.URL, "/") == 1 {
			m.URL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", m.URL)

			// Convert "https://github.com/owner/repo" to API path.
			// Don't modify URLs that already point to the API.
		} else if !strings.Contains(m.URL, "api.github") {
			m.URL = strings.Split(m.URL, "github.")[1]
			// split "com/owner/repo" on "/" and keep two substrings, "com" and "owner/repo"
			m.URL = strings.SplitN(m.URL, "/", 2)[1]
			m.URL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", m.URL)
		}
	}

	if len(m.URLCommands) != 0 {
		m.URLCommands.setDefaults(defaults)
	}
}

// setDefaults sets the defaults for undefined Monitor values using defaults.
func (c *URLCommandSlice) setDefaults(defaults Defaults) {
	for index := range *c {
		(*c)[index].setDefaults(defaults)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (c *URLCommand) setDefaults(defaults Defaults) {
	// Default IgnoreMiss
	if c.IgnoreMiss == "" {
		c.IgnoreMiss = defaults.Monitor.IgnoreMiss
	} else if strings.ToLower(c.IgnoreMiss) == "true" || strings.ToLower(c.IgnoreMiss) == "yes" {
		c.IgnoreMiss = "y"
	} else {
		c.IgnoreMiss = "n"
	}
}

// setVersion sets Monitor.Version to v.
func (m *Monitor) setVersion(v string) {
	m.status.version = v
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

// getAtIndex returns the character at index of str
func getAtIndex(str string, index int) string {
	return str[index : index+1]
}

// query queries the Monitor source, updating Monitor.Version
// and returning true if it has changed (is a new release),
// otherwise returns false.
func (m *Monitor) query(i int) bool {
	customTransport := &http.Transport{}
	// HTTPS insecure skip verify.
	if m.AllowInvalidCerts == "y" {
		customTransport = http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	req, err := http.NewRequest(http.MethodGet, m.URL, nil)
	if err != nil {
		log.Printf("ERROR: %s, %s", m.ID, err)
		return false
	}

	if m.AccessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", m.AccessToken))
	}

	client := &http.Client{Transport: customTransport}
	resp, err := client.Do(req)

	if err != nil {
		// Don't crash on invalid certs.
		if strings.Contains(err.Error(), "x509") {
			log.Printf("WARNING: x509 for %s (Cert invalid)", m.ID)
			return false
		}
		log.Printf("ERROR: %s, %s", m.ID, err)
		return false
	}

	// Read the response body.
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: %s, %s", m.ID, err)
		return false
	}
	// Convert the body to string.
	body := string(rawBody)
	version := body

	// GitHub monitor.
	if m.Type == "github" {
		// Check for rate limit.
		if len(body) < 500 {
			if strings.Contains(body, "rate limit") {
				if *verbose {
					log.Printf("WARNING: Rate limit reached on %s", m.ID)
				}
				return false
			}
		}
		version = strings.Split(body, `"tag_name"`)[1]
		version = strings.Split(version, ",")[0]
		version = strings.Split(version, `"`)[1]
		// Raw URL Monitor.
	} else {
		// Iterate through the commands to filter out the version.
		for _, command := range m.URLCommands {
			versionBak := version
			switch command.Type {
			case "split":
				versions := strings.Split(version, command.Text)

				if len(versions) == 1 {
					if getAtIndex(m.status.monitorMisses, 0) == "0" {
						log.Printf("WARNING: %s, %s didn't find any '%s' to split on", m.ID, command.Type, command.Text)
						m.status.monitorMisses = replaceAtIndex(m.status.monitorMisses, '1', 0)
					}
					// Stop if miss
					if command.IgnoreMiss == "n" {
						return false
					}
					// Ignore Misses
					version = versionBak
					continue
				}

				index := command.Index
				// Handle negative indices.
				if index < 0 {
					index = len(versions) + index
				}

				if (len(versions) - index) < 1 {
					if getAtIndex(m.status.monitorMisses, 1) == "0" {
						log.Printf("WARNING: %s, %s (%s) returned %d elements but the index wants element number %d", m.ID, command.Type, command.Text, len(versions), (index + 1))
						m.status.monitorMisses = replaceAtIndex(m.status.monitorMisses, '1', 1)
					}
					// Stop if miss
					if command.IgnoreMiss == "n" {
						return false
					}
					// Ignore Misses
					version = versionBak
					continue
				}

				version = versions[index]
			case "replace":
				version = strings.ReplaceAll(version, command.Old, command.New)
			case "regex", "regex_submatch":
				re := regexp.MustCompile(command.Regex)

				var versions []string
				if command.Type == "regex" {
					versions = re.FindAllString(version, -1)
				} else if command.Type == "regex_submatch" {
					if command.Index < 0 {
						log.Printf("WARNING: %s, %s (%s) shouldn't use negative indices as the array is always made up from the first match.", m.ID, command.Type, command.Regex)
					}
					versions = re.FindStringSubmatch(version)
				}

				if len(versions) == 0 {
					if getAtIndex(m.status.monitorMisses, 2) == "0" {
						log.Printf("WARNING: %s, %s (%s) didn't return any matches", m.ID, command.Type, command.Regex)
						m.status.monitorMisses = replaceAtIndex(m.status.monitorMisses, '1', 2)
					}
					// Stop if miss
					if command.IgnoreMiss == "n" {
						return false
					}
					// Ignore Misses
					version = versionBak
					continue
				}

				index := command.Index
				// Handle negative indices.
				if command.Index < 0 {
					index = len(versions) + command.Index
				}

				if (len(versions) - index) < 1 {
					if getAtIndex(m.status.monitorMisses, 3) == "0" {
						log.Printf("WARNING: %s, %s (%s) returned %d elements but the index wants element number %d", m.ID, command.Type, command.Regex, len(versions), (index + 1))
						m.status.monitorMisses = replaceAtIndex(m.status.monitorMisses, '1', 3)
					}
					// Stop if miss
					if command.IgnoreMiss == "n" {
						return false
					}
					// Ignore Misses
					version = versionBak
					continue
				}

				version = versions[index]
			default:
				log.Printf("ERROR: %s, %s is an unknown type for url_commands", m.ID, command.Type)
				continue
			}
		}
	}
	// If this version is different (new).
	if version != m.status.version {
		// Check for a regex match in the body if one is desired.
		if m.RegexContent != "" {
			regexMatch := regexCheckContent(m.RegexContent, body, version)
			if !regexMatch {
				m.status.regexMissesContent++
				if *verbose && m.status.regexMissesContent == 1 {
					log.Printf("INFO: %s, Regex not matched on content for version %s", m.ID, version)
				}
				return false
			}
		}
		// Check that the version grabbed satisfies the specified regex (if there is any).
		if m.RegexVersion != "" {
			regexMatch := regexCheck(m.RegexVersion, version)
			if !regexMatch {
				m.status.regexMissesVersion++
				if *verbose && m.status.regexMissesVersion == 1 {
					log.Printf("INFO: %s, Regex not matched on version %s", m.ID, version)
				}
				return false
			}
		}

		// Found new version, so reset regex misses
		m.status.regexMissesContent = 0
		m.status.regexMissesVersion = 0

		// First version found.
		if m.status.version == "" {
			m.setVersion(version)
			log.Printf("INFO: %s, Starting Release - %s", m.ID, version)
			// Don't notify on first version.
			return false
		}

		// New version found.
		m.setVersion(version)
		log.Printf("INFO: %s, New Release - %s", m.ID, version)
		return true
	}

	// No version change.
	return false
}
