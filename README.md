# Project Moved
This project has been renamed to Hymenaios and moved to [hymenaios-io/Hymenaios](https://github.com/hymenaios-io/Hymenaios). Head over there and you should be able to find a demo of what it looks like/does.

# Release-Notifier
[![Go Report Card](https://goreportcard.com/badge/github.com/JosephKav/Release-Notifier)](https://goreportcard.com/report/github.com/JosephKav/Release-Notifier)
[![Build](https://github.com/JosephKav/Release-Notifier/actions/workflows/build.yml/badge.svg)](https://github.com/JosephKav/Release-Notifier/actions/workflows/build.yml)
[![GitHub Release](https://img.shields.io/github/release/JosephKav/Release-Notifier.svg?logo=github&style=flat-square&color=blue)](https://github.com/JosephKav/Release-Notifier/releases)

Release-Notifier will query websites at a user defined interval for new software releases and then trigger Gotify/Slack/WebHook notification(s) when one has been found.
For example, you could set it to monitor the Gitea repo ([go-gitea/gitea](https://github.com/go-gitea/gitea)). This will query the [GitHub API](https://api.github.com/repos/go-gitea/gitea/releases/latest) and track the "tag_name" variable. When this variable changes from what it was on a previous query, an AWX 'GitHub' WebHook could be triggered to update Gitea on your server.

##### Table of Contents
- [Release-Notifier](#release-notifier)
  * [Output](#output)
  * [Command-line arguments](#command-line-arguments)
  * [Config formatting](#config-formatting)
      - [Example](#example)
      - [Defaults](#defaults)
        * [Example](#example-1)
        * [Service](#defaults---service)
        * [Gotify](#defaults---gotify)
        * [Slack](#defaults---slack)
        * [WebHook](#defaults---webhook)
      - [Monitor](#monitor)
        * [Example](#example-2)
        * [Service](#monitor---service)
        * [Gotify](#monitor---gotify)
        * [Slack](#monitor---slack)
        * [WebHook](#monitor---webhook)

## Output
![image](https://user-images.githubusercontent.com/4267227/138481247-cbee6073-bf6c-4be2-8b2e-875f3719e738.png)
```bash
$ release_notifier -config myConfig.yml -loglevel 3 -timestamps
2021/11/08 02:18:24 VERBOSE: Loading config from 'config.yml'
2021/11/08 02:18:24 INFO: 4 targets with 5 services to monitor:
2021/11/08 02:18:24   - goauthentik/authentik
2021/11/08 02:18:24   - ansible/awx-operator
2021/11/08 02:18:24   - CV-Site:
2021/11/08 02:18:24       - gohugoio/hugo
2021/11/08 02:18:24       - adnanh/webhook
2021/11/08 02:18:24   - louislam/uptime-kuma
2021/11/08 02:18:24 VERBOSE: Tracking goauthentik/authentik at https://api.github.com/repos/goauthentik/authentik/releases/latest every 10m
2021/11/08 02:18:24 INFO: goauthentik/authentik (Authentik), Starting Release - 2021.10.2
2021/11/08 02:18:35 VERBOSE: Tracking ansible/awx-operator at https://api.github.com/repos/ansible/awx-operator/releases/latest every 10m
2021/11/08 02:18:35 INFO: ansible/awx-operator (AWX), Starting Release - 0.14.0
2021/11/08 02:18:52 VERBOSE: Tracking gohugoio/hugo at https://api.github.com/repos/gohugoio/hugo/releases/latest every 10m
2021/11/08 02:18:52 INFO: gohugoio/hugo (CV-Site), Starting Release - 0.89.1
2021/11/08 02:19:09 VERBOSE: Tracking adnanh/webhook at https://api.github.com/repos/adnanh/webhook/releases/latest every 10m
2021/11/08 02:19:09 INFO: adnanh/webhook (CV-Site), Starting Release - 2.8.0
2021/11/08 02:19:28 VERBOSE: Tracking louislam/uptime-kuma at https://api.github.com/repos/louislam/uptime-kuma/releases/latest every 10m
2021/11/08 02:19:28 INFO: louislam/uptime-kuma (Uptime-Kuma), Starting Release - 1.10.0
2021/11/08 10:11:58 INFO: louislam/uptime-kuma (Uptime-Kuma), New Release - 1.10.1
2021/11/08 10:11:58 INFO: louislam/uptime-kuma (Uptime-Kuma), Slack message sent
2021/11/08 10:11:59 INFO: louislam/uptime-kuma (Uptime-Kuma), (202) WebHook received
2021/11/08 16:09:35 INFO: gohugoio/hugo (CV-Site), New Release - 0.89.2
2021/11/08 16:09:35 INFO: gohugoio/hugo (CV-Site), Slack message sent
2021/11/08 16:09:40 ERROR: gohugoio/hugo (CV-Site), WebHook
Post "https://awx.example.io/api/v2/job_templates/30/github/": context deadline exceeded
2021/11/08 16:09:50 INFO: gohugoio/hugo (CV-Site), (202) WebHook received
2021/11/08 20:08:52 INFO: goauthentik/authentik (Authentik), New Release - 2021.10.3
2021/11/08 20:08:52 INFO: goauthentik/authentik (Authentik), Sleeping for 2h before sending the WebHook
2021/11/08 20:08:52 INFO: goauthentik/authentik (Authentik), Slack message sent
2021/11/08 22:08:57 ERROR: goauthentik/authentik (Authentik), WebHook
Post "https://awx.example.io/api/v2/job_templates/40/github/": context deadline exceeded
2021/11/08 22:09:07 INFO: goauthentik/authentik (Authentik), (202) WebHook received
2021/11/09 14:52:40 INFO: louislam/uptime-kuma (Uptime-Kuma), New Release - 1.10.2
2021/11/09 14:52:40 INFO: louislam/uptime-kuma (Uptime-Kuma), Slack message sent
2021/11/09 14:52:41 INFO: louislam/uptime-kuma (Uptime-Kuma), (202) WebHook received
```

## Command-line arguments
```bash
$ release_notifier -h
Usage of /usr/local/bin/release_notifier:
  -config string
        The path to the config file to use (default "config.yml")
  -loglevel int
        0 = error, 1 = warn,
        2 = info,  3 = verbose,
        4 = debug (default 2)
```

## Config formatting
#### Example
```yaml
defaults:
  service:
    interval: 10m
    access_token: 'GITHUB_ACCESS_TOKEN'
    progressive_versioning: true
    allow_invalid: false
    ignore_misses: false
  gotify:
    priority: 5
    title: 'Release notifier'
    message: '${service_id} - ${version} released'
    delay: 0s
    max_tries: 3
    extras:
      android_action: '${service_url}'
      client_display: 'text/plain'
      client_notification: '${service_url}'
  slack:
    message: '<${service_url}|${service_id}> - ${version} released'
    username: 'Release Notifier'
    icon_url: 'https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png'
    delay: 0s
    max_tries: 3
  webhook:
    desired_status_code: 0
    delay: 1h2m3s
    max_tries: 3
    silent_fails: false
monitor:
  - id: CV-Site
    service:
      - id: golang/go
        type: url
        url: https://github.com/golang/go/releases
        url_commands:
          - type: regex
            regex: 'go[0-9.]+"'
            index: 0
          - type: regex
            regex: '[0-9.]+[0-9]'
            index: 0
      - type: github
        url: adnanh/webhook
        interval: 300s
    webhook:
      type: github
      url: https://AWX_HOSTNAME/api/v2/job_templates/35/github/
      secret: SECRET_KEY
    gotify:
      url: https://GOTIFY_URL
      token: APP_TOKEN
      delay: 5s
    slack:
      url: https://SLACK_INCOMING_WEBHOOK
      delay: 5s
```
Above, I set defaults.service.interval to 10m and then don't define an interval for the golang/go service. Therefore, the monitor for this service will follow defaults.service.interval and query the site every 10m. But, I can override that interval by stating it, for example in the adnanh/webhook service, I have set interval to 300s and so that page will be queried every 300 seconds. The defaults.webhook.delay of 1h5m4s will delay sending the webhook by 1 hour, 2 minutes and 3 seconds when a version change is noticed. The CV-Site Slack and Gotify messages will be delayed by 5 seconds since the slack.delay overrides defaults.slack.delay. The defaults.gotify.extras will cause the notifications to open the monitor.service.url when clicked and if the Gotify app is in focus when the notification is received.

#### Defaults
Defaults are not required in the config, but you can override the coded defaults with your own defaults for service, gotify, slack and webhook in this defaults section. You could for example have a tiny defaults section that only has `defaults -> slack -> username: 'USERNAME'`, you do not need to define all values for a section. In the examples below, the values set are the coded defaults that will be used if they haven't been included in the service being monitored or the config defaults. (excluding access_token)

##### Example
```yaml
defaults:
  service:
    interval: 10m
    access_token: 'GITHUB_ACCESS_TOKEN'
    progressive_versioning: true
    allow_invalid: false
    ignore_misses: false
  gotify:
    delay: 0s
    max_tries: 3
    message: '${service_id} - ${version} released'
    priority: 5
    title: 'Release notifier'
    extras:
      android_action: ''
      client_display: 'text/plain'
      client_notification: ''
  slack:
    message: '<${service_url}|${service_id}> - ${version} released'
    username: 'Release Notifier'
    icon_emoji: ':github:'
    icon_url: ''
    delay: 0s
    max_tries: 3
  webhook:
    desired_status_code: 0
    delay: 0s
    max_tries: 3
    silent_fails: false
```

##### Defaults - Monitor
```yaml
defaults:
  service:
    interval: 10m                       # Time between monitor queries.
    access_token: 'GITHUB_ACCESS_TOKEN' # Increase API rate limit with an access token (and allow querying private repos). Used when type="github".
    progressive_versioning: true        # Only send Slack(s) and/or WebHook(s) when the version increases (semantic versioning - e.g. v1.2.3a).
    allow_invalid: false                # Allow invalid HTTPS Certificates.
    ignore_misses: false                # Ignore url_command fails (e.g. split on text that doesn't exist)
```

##### Defaults - Gotify
```yaml
defaults:
  gotify:
    delay: 0s                                      # The delay before sending messages.
    max_tries: 3                                   # Number of times to resend until a 2XX status code is received.
    message: '${service_id} - ${version} released' # Formatting of the message to send.
    priority: 5                                    # Priority of the message.
    title: 'Release notifier'                      # Title of the message.
    extras:
      android_action: ''                           # URL to open when a notification is received whilst GOtify is in focus.
      client_display: 'text/plain'                 # Whether the message should be rendered in markdown or plain text.
      client_notification: ''                      # URL to open when the notification is clicked (Android).
```
message - Each element of the service array of a monitor element will trigger a Gotify message to the gotify's of the parent monitor unless service.skip_gotify is True. This is the message that is sent when a change in version is noticed.
- `${service_id}`  will be replaced with the ID.
- `${service_url}` will be replaced with the URL
- `${version}`     will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${monitor_id}`  will be replaced with the ID given to the parent (monitor element).

extras:
- `${service_url}` will be replaced with the URL

(of the service element that is triggering the message)

##### Defaults - Slack
```yaml
defaults:
  slack:
    message: '<${service_url}|${service_id}> - ${version} released' # Formatting of the message to send.
    username: 'Release Notifier'                                    # The user to message as.
    icon_emoji: ':github:'                                          # The emoji icon for that user.
    icon_url: ''                                                    # The URL of an icon for that user.
    delay: 0s                                                       # The delay before sending messages.
    max_tries: 3                                                     # Number of times to resend until a 2XX status code is received.
```
message - Each element of the service array of a monitor element will trigger a Slack message to the slack's of the parent monitor unless service.skip_slack is True. This is the message that is sent when a change in version is noticed.
- `${service_id}`  will be replaced with the ID.
- `${service_url}` will be replaced with the URL
- `${version}`     will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${monitor_id}`  will be replaced with the ID given to the parent (monitor element).

(of the service element that is triggering the message)

##### Defaults - WebHook
```yaml
defaults:
  webhook:
    desired_status_code: 0 # Status code indicating a success. Send max_tries # of times until we receive this. 0 = accept any 2XX code.
    delay: 0s              # The delay before sending webhooks.
    max_tries: 3            # Number of times to resend until desired_status_code is received.
    silent_fails: false    # Whether to notify Slack if a webhook fails max_tries times
```

#### Monitor
##### Example
```yaml
monitor:
  - id: CV-Site
    service:
      - id: golang/go
        type: url
        url: https://github.com/golang/go/releases
        url_commands:
          - type: split
            text: '.zip'
            index: 0
          - type: split
            text: 'go'
            index: -1
      - type: github
        url: adnanh/webhook
        interval: 345s
    slack:
      - url: https://SLACK_INCOMING_WEBHOOK
        delay: 5h5m5m
      - url: https://OTHER_SLACK_INCOMING_WEBHOOK
        delay: 1h1m1m
    webhook:
      - type: github
        url: https://AWX_HOSTNAME/api/v2/job_templates/35/github/
        secret: SECRET_KEY
        delay: 5h5m5m
      - type: github
        url: https://OTHER_AWX_HOSTNAME/api/v2/job_templates/7/github/
        secret: OTHER_SECRET_KEY
        delay: 1h5m5m
  - id: Gitea
    service:
      type: github
      url: go-gitea/gitea
    gotify:
      url: https://GOTIFY_URL
      token: APP_TOKEN
    webhook:
      type: github
      url: https://AWX_HOSTNAME/api/v2/job_templates/36/github/
      secret: SECRET
      delay: 5h5m5m
```
This program can monitor multiple services. Just provide them in the standard yaml list format like above. It can also monitor multiple sites under the same monitor element (if the same style Slack/Gotify message(s) and webhook(s) are wanted), and can send multiple Slack/Gotify messages, and/or github style webhooks when a new release is found. Just turn those sections into yaml lists. Note, if you don't have multiple slack messages, multiple webhooks or multiple sites/repos under the same service, you don't have to format it as a list. This can be seen in the Gitea example above (they don't all need to be lists. e.g. you could have a list of monitors with a single webhook that isn't formatted as a list).

##### Monitor - Service
```yaml
monitor:
  - id: "PRETTY_MONITOR_NAME" # Optional. Replaces ${monitor_id} in Slack messages.
    service:                  # Required.
      id: "PRETTY NAME"                                # Optional. Used in logs/Slack messages.
      type: "github"|"url"                             # Optional. If unset, ill be set to github if only one / is present, otherwise url.
      url: GITHUB_OWNER/REPO                           # Required. URL/Repo to monitor. "OWNER/REPO" if type="github" | "URL_TO_MONITOR" if type="url"
      url_commands:                                    # Optional. Used when type="url" as a list of commands to filter out the release from the URL content.
        - type: "regex"|"regex_submatch"|"replace"|"split" # Required. Type of command to filter release with.
          regex: 'grafana\/tree\/v[0-9.]+"'                # Required if type=("regex"|"regex_submatch"). Regex to split URL content on.
          index: -1                                        # Required if type=("regex"|"regex_submatch"|"split"). Take this index of the split data. (supports negative indices).
          old: "TEXT_TO_REPLACE"                           # Required if type="replace". Replace this text.
          new: "REPLACE_WITH_THIS"                         # Required if type="replace". Replace with this text.
          text: "ABC"                                      # Required if type="split". Split on this text.
          ignore_misses: false                             # Optional. Ignore fails (e.g. split on text that doesn't exist or no regex match)
      regex_content: "abc-[a-z]+-${version}_amd64.deb" # Optional. This regex must exist on the URL content to be classed as a new release.
      regex_version: '^v[0-9.]+$'                      # Optional. The version found must contain matching regex to be classed as a new release.
      progressive_versioning: true                     # Optional. # Only send Slack(s) and/or WebHook(s) when the version increases (semantic versioning - e.g. v1.2.3a).
      allow_invalid: false                             # Optional. Allow invalid HTTPS Certificates.
      access_token: 'GITHUB_ACCESS_TOKEN'              # Optional. GitHub access token to use. Allows smaller interval (higher API rate limit).
      skip_gotify: false                               # Optional. Don't send Gotify messages for new releases of this service.
      skip_slack: false                                # Optional. Don't send Slack messages for new releases of this service.
      skip_webhook: false                              # Optional. Don't send WebHooks for new releases of this service.
      interval: 10m                                    # Optional. The duration (AhBmCs where h is hours, m is minutes and s is seconds) to sleep between querying the URL for the version.
```
The values of the optional boolean arguments are the default values.

regex_content:
- `${version}` will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${version_no_v}` will be replaced with the version that was found where any v's in the version are removed (e.g. `${version} = v10.6.3` - `${version_no_v} = 10.6.3`).

regex_version:
- Remember `^` indicates the start of the string. A regex of `v[0-9.]` would find a match on `betav0.5`. Adding the `^` at the start would mean that version doesn't match the regex.
- Remember `$` indicates the end of the string. A regex of `v[0-9.]` would find a match on `v0.5-beta`. Adding the `$` at the end would mean that version doesn't match the regex.

url_commands:
- type:
  - regex:
    - This allows you to extract the version with regex. The whole regex match will be returned as the version. This needs an `index` to define which match to return (e.g. `-1` would be the last match).
  - regex_submatch:
    - This uses a `regex` such as `Proxmox VE ([0-9.]+) ISO Installer`. When used on a URL with `Proxmox VE 6.4 ISO Installer` in the content, with an `index` of `1`, the match in the brackets will be returned(`6.4` in this case).
  - split:
    - This will split the string on `text` and use the element at `index`.
  - replace:
    - This will replace `old` with `new` in the URL content at this point.

##### Monitor - Gotify
```yaml
monitor:
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${monitor_id} in Slack messages.
    service:                  # Required.
      ....
    gotify:                    # Optional.
      url: https://GOTIFY_URL                        # Required. The URL of the Gotify server.
      token: APP_TOKEN                               # Required. The app token to use.
      delay: 0s                                      # Optional. The delay before sending messages.
      max_tries: 3                                   # Optional. Number of times to resend until a 2XX status code is received.
      message: '${service_id} - ${version} released' # Optional. Formatting of the message to send.
      priority: 5                                    # Optional. Priority of the message.
      title: 'Release notifier'                      # Optional. Title of the message.
      extras:
        android_action: ''                           # Optional. URL to open when a notification is received whilst GOtify is in focus.
        client_display: 'text/plain'                 # Optional. Whether the message should be rendered in markdown or plain text. (Must be either 'text/plain' or 'text/markdown')
        client_notification: ''                      # Optional. URL to open when the notification is clicked (Android).
```
The values of the optional arguments are the default values.

message:
- `${service_id}`  will be replaced with the ID.
- `${service_url}` will be replaced with the URL.
- `${version}`     will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${monitor_id}`  will be replaced with the ID given to the parent (monitor element).

extras:
- `${service_url}` will be replaced with the URL.

(of the service element that is triggering the message)

##### Monitor - Slack
```yaml
monitor:
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${monitor_id} in Slack messages.
    service:                  # Required.
      ....
    slack:                    # Optional.
      url: "SLACK_INCOMING_WEBHOOK"                                   # Required. The URL of the incoming Slack WebHook to send the message to.
      message: '<${service_url}|${service_id}> - ${version} released' # Optional. Formatting of the message to send.
      username: 'Release Notifier'                                    # Optional. The user to message as.
      icon_emoji: ':github:'                                          # Optional. The emoji icon for that user.
      icon_url: ''                                                    # Optional. The URL of an icon for that user.
      delay: '0s'                                                     # Optional. The duration (AhBmCs where h is hours, m is minutes and s is seconds) to delay sending the message by.
      max_tries: 3                                                     # Optional. The number of times to resend until a 2XX status code is received.
```
The values of the optional arguments are the default values.

message:
- `${service_id}`  will be replaced with the ID.
- `${service_url}` will be replaced with the URL.
- `${version}`     will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${monitor_id}`  will be replaced with the ID given to the parent (monitor element).

(of the service element that is triggering the message)

##### Monitor - WebHook
```yaml
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${service} in Slack messages.
    service:                  # Required.
      ...
    webhook:                  # Optional.
      type: "github"         # Required. The type of WebHook to send (Currently only github is supported).
      url: "WEBHOOK_URL"     # Required. The URL to send the WebHook to.
      secret: "SECRET"       # Required. The secret to send the WebHook with.
      desired_status_code: 0 # Optional. Keep sending the WebHooks until we recieve this status code (0 = accept any 2XX code).
      delay: '0s'            # Optional. The duration (AhBmCs where h is hours, m is minutes and s is seconds) to delay sending the WebHook by.
      max_tries: 3            # Optional. Number of times to try re-sending WebHooks until we receive desired_status_code
      silent_fails: false    # Optional. Whether to send Slack messages to the Slacks of the Monitor when a WebHook fails max_tries times.
```
The values of the optional arguments are the default values.
