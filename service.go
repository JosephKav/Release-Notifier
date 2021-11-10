package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
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
	RegexContent          string          `yaml:"regex_content"`          // "abc-[a-z]+-${version}_amd64.deb" This regex must exist in the body of the URL to trigger new version actions.
	RegexVersion          string          `yaml:"regex_version"`          // "v*[0-9.]+" The version found must match this release to trigger new version actions.
	ProgressiveVersioning string          `yaml:"progressive_versioning"` // default - true  = Version has to be greater than the previous to trigger Slack(s)/WebHook(s).
	AllowInvalidCerts     string          `yaml:"allow_invalid"`          // default - false = Disallows invalid HTTPS certificates.
	AccessToken           string          `yaml:"access_token"`           // GitHub access token to use.
	Interval              uint            `yaml:"interval"`               // 600 = Sleep 600 seconds between queries.
	SkipSlack             bool            `yaml:"skip_slack"`             // default - false = Don't skip Slack messages for new releases.
	SkipWebHook           bool            `yaml:"skip_webhook"`           // default - false = Don't skip WebHooks for new releases.
	IgnoreMiss            string          `yaml:"ignore_misses"`          // Ignore URLCommands that fail (e.g. split on text that doesn't exist)
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

// checkValues will check the variables for all of this services Slack recipients.
func (s *ServiceSlice) checkValues(serviceID string) {
	for index := range *s {
		(*s)[index].checkValues(serviceID, index)
	}
}

// checkValues will check that the variables are valid for this Slack recipient.
func (s *Service) checkValues(serviceID string, index int) {
	if s.Slack.Delay != "" {
		_, err := time.ParseDuration(s.Slack.Delay)
		if err != nil {
			fmt.Printf("ERROR: %s.slack[%d].delay (%s) is invalid (Use 'AhBmCs' duration format)", serviceID, index, s.Slack.Delay)
			os.Exit(1)
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

// setDefaults sets the defaults for undefined Service values using defaults.
func (s *status) init() {
	s.serviceMisses = "0000"
}

// setDefaults sets the defaults for undefined Service values using defaults.
func (s *ServiceSlice) setDefaults(defaults Defaults) {
	for index := range *s {
		(*s)[index].setDefaults(defaults)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (s *Service) setDefaults(defaults Defaults) {
	// Default GitHub Access Token.
	if s.AccessToken == "" {
		s.AccessToken = defaults.Service.AccessToken
	}

	// Default allowance/rejection of invalid certs.
	if s.AllowInvalidCerts == "" {
		s.AllowInvalidCerts = defaults.Service.AllowInvalidCerts
	} else if strings.ToLower(s.AllowInvalidCerts) == "true" || strings.ToLower(s.AllowInvalidCerts) == "yes" {
		s.AllowInvalidCerts = "y"
	} else {
		s.AllowInvalidCerts = "n"
	}

	// Default progressive versioning (versions have to be successive to notify)
	if s.ProgressiveVersioning == "" {
		s.ProgressiveVersioning = defaults.Service.ProgressiveVersioning
	} else if strings.ToLower(s.ProgressiveVersioning) == "false" || strings.ToLower(s.ProgressiveVersioning) == "no" {
		s.ProgressiveVersioning = "n"
	} else {
		s.ProgressiveVersioning = "y"
	}

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
	if s.Interval == 0 {
		s.Interval = defaults.Service.Interval
	}

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

	if s.IgnoreMiss == "" {
		s.IgnoreMiss = defaults.Service.IgnoreMiss
	} else if strings.ToLower(s.IgnoreMiss) == "true" || strings.ToLower(s.IgnoreMiss) == "yes" {
		s.IgnoreMiss = "y"
	} else {
		s.IgnoreMiss = "n"
	}

	s.URLCommands.setDefaults(defaults, s)
}

// setDefaults sets the defaults for undefined Service values using defaults.
func (c *URLCommandSlice) setDefaults(defaults Defaults, service *Service) {
	for index := range *c {
		(*c)[index].setDefaults(defaults, service)
	}
}

// setDefaults sets the defaults for each undefined var using defaults.
func (c *URLCommand) setDefaults(defaults Defaults, service *Service) {
	// Default IgnoreMiss.
	if c.IgnoreMiss == "" {
		c.IgnoreMiss = service.IgnoreMiss
	} else if strings.ToLower(c.IgnoreMiss) == "true" || strings.ToLower(c.IgnoreMiss) == "yes" {
		c.IgnoreMiss = "y"
	} else {
		c.IgnoreMiss = "n"
	}
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
		log.Printf("ERROR: %s, %s", s.ID, err)
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
			if *logLevel > 0 {
				log.Printf("WARNING: x509 for %s (%s) (Cert invalid)", s.ID, monitorID)
			}
			return false
		}
		log.Printf("ERROR: %s (%s), %s", s.ID, monitorID, err)
		return false
	}

	// Read the response body.
	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: %s (%s), %s", s.ID, monitorID, err)
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
				if strings.Contains(body, "Bad credentials") {
					log.Println("ERROR: GitHub Access Token is invalid!")
					os.Exit(1)
				}
				log.Printf("ERROR: tag_name not found for %s (%s) at %s\n%s", s.ID, monitorID, s.URL, body)
				return false
			}
			if strings.Contains(body, "rate limit") {
				if *logLevel > 0 {
					log.Printf("WARNING: Rate limit reached on %s (%s)", s.ID, monitorID)
				}
				return false
			}
		}
		version = strings.Split(body, `"tag_name"`)[1]
		version = strings.Split(version, ",")[0]
		version = strings.Split(version, `"`)[1]
		// Raw URL Service.
	}

	// Iterate through the commands to filter out the version.
	for _, command := range s.URLCommands {
		versionBak := version
		if *logLevel > 3 {
			log.Printf("DEBUG: Looking through %s", version)
		}
		switch command.Type {
		case "split":
			versions := strings.Split(version, command.Text)

			if len(versions) == 1 {
				if getAtIndex(s.status.serviceMisses, 0) == "0" {
					if *logLevel > 0 {
						log.Printf("WARNING: %s (%s), %s didn't find any '%s' to split on", s.ID, monitorID, command.Type, command.Text)
					}
					s.status.serviceMisses = replaceAtIndex(s.status.serviceMisses, '1', 0)
				}
				// Stop if miss.
				if command.IgnoreMiss == "n" {
					return false
				}
				// Ignore Misses.
				version = versionBak
				continue
			}

			index := command.Index
			// Handle negative indices.
			if index < 0 {
				index = len(versions) + index
			}

			if (len(versions) - index) < 1 {
				if getAtIndex(s.status.serviceMisses, 1) == "0" {
					if *logLevel > 0 {
						log.Printf("WARNING: %s (%s), %s (%s) returned %d elements but the index wants element number %d", s.ID, monitorID, command.Type, command.Text, len(versions), (index + 1))
					}
					s.status.serviceMisses = replaceAtIndex(s.status.serviceMisses, '1', 1)
				}
				// Stop if miss.
				if command.IgnoreMiss == "n" {
					return false
				}
				// Ignore Misses.
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
					if *logLevel > 0 {
						log.Printf("WARNING: %s (%s), %s (%s) shouldn't use negative indices as the array is always made up from the first match.", s.ID, monitorID, command.Type, command.Regex)
					}
				}
				versions = re.FindStringSubmatch(version)
			}

			if len(versions) == 0 {
				if getAtIndex(s.status.serviceMisses, 2) == "0" {
					if *logLevel > 0 {
						log.Printf("WARNING: %s (%s), %s (%s) didn't return any matches", s.ID, monitorID, command.Type, command.Regex)
					}
					s.status.serviceMisses = replaceAtIndex(s.status.serviceMisses, '1', 2)
				}
				// Stop if miss.
				if command.IgnoreMiss == "n" {
					return false
				}
				// Ignore Misses.
				version = versionBak
				continue
			}

			index := command.Index
			// Handle negative indices.
			if command.Index < 0 {
				index = len(versions) + command.Index
			}

			if (len(versions) - index) < 1 {
				if getAtIndex(s.status.serviceMisses, 3) == "0" {
					if *logLevel > 0 {
						log.Printf("WARNING: %s (%s), %s (%s) returned %d elements but the index wants element number %d", s.ID, monitorID, command.Type, command.Regex, len(versions), (index + 1))
					}
					s.status.serviceMisses = replaceAtIndex(s.status.serviceMisses, '1', 3)
				}
				// Stop if miss.
				if command.IgnoreMiss == "n" {
					return false
				}
				// Ignore Misses.
				version = versionBak
				continue
			}

			version = versions[index]
		default:
			log.Printf("ERROR: %s (%s), %s is an unknown type for url_commands", s.ID, monitorID, command.Type)
			continue
		}
		if *logLevel > 3 {
			log.Printf("DEBUG: %s (%s), Resolved to %s", s.ID, monitorID, version)
		}
	}

	// If this version is different (new).
	if version != s.status.version {
		// Check for a progressive change in version.
		if s.ProgressiveVersioning == "y" && s.status.version != "" {
			failedSemanticVersioning := false
			oldVersion, err := semver.NewVersion(s.status.version)
			if err != nil {
				log.Printf("ERROR: %s (%s), failed converting '%s' to a semantic version", s.ID, monitorID, s.status.version)
				failedSemanticVersioning = true
			}
			newVersion, err := semver.NewVersion(version)
			if err != nil {
				log.Printf("ERROR: %s (%s), failed converting '%s' to a semantic version", s.ID, monitorID, version)
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
				s.status.regexMissesContent++
				if *logLevel > 2 && s.status.regexMissesContent == 1 {
					log.Printf("VERBOSE: %s (%s), Regex not matched on content for version %s", s.ID, monitorID, version)
				}
				return false
			}
		}
		// Check that the version grabbed satisfies the specified regex (if there is any).
		if s.RegexVersion != "" {
			regexMatch := regexCheck(s.RegexVersion, version)
			if !regexMatch {
				s.status.regexMissesVersion++
				if *logLevel > 2 && s.status.regexMissesVersion == 1 {
					log.Printf("VERBOSE: %s (%s), Regex not matched on version %s", s.ID, monitorID, version)
				}
				return false
			}
		}

		// Found new version, so reset regex misses.
		s.status.regexMissesContent = 0
		s.status.regexMissesVersion = 0

		// First version found.
		if s.status.version == "" {
			if s.ProgressiveVersioning == "y" {
				_, err := semver.NewVersion(version)
				if err != nil {
					log.Printf("ERROR: %s (%s), failed converting '%s' to a semantic version. If all versions are in this style, consider adding url_commands to get the version into the style of '1.2.3a' (https://semver.org/), or disabling progressive versioning (globally with defaults.service.progressive_versioning or just for this service with the progressive_versioning var)", s.ID, monitorID, version)
					return false
				}
			}

			s.setVersion(version)
			if *logLevel > 1 {
				log.Printf("INFO: %s (%s), Starting Release - %s", s.ID, monitorID, version)
			}
			// Don't notify on first version.
			return false
		}

		// New version found.
		s.setVersion(version)
		if *logLevel > 1 {
			log.Printf("INFO: %s (%s), New Release - %s", s.ID, monitorID, version)
		}
		return true
	}

	// No version change.
	return false
}
