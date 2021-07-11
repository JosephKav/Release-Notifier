# Release-Notifier

Release-Notifier will query websites at a user defined interval for new software releases and then trigger a WebHook/Slack notification when one has been found.
For example, you could set it to monitor the Gitea repo ([go-gitea/gitea](https://github.com/go-gitea/gitea)). This will query the [GitHub API](https://api.github.com/repos/go-gitea/gitea/releases/latest) and track the "tag_name" variable. When this variable changes from what it was on a previous query, an AWX 'GitHub' WebHook could be triggered to update Gitea on your server.

##### Table of Contents
- [Release-Notifier](#release-notifier)
  * [Output](#output)
  * [Command-line arguments](#command-line-arguments)
  * [Config formatting](#config-formatting)
      - [Example](#example)
      - [Defaults](#defaults)
        * [Example](#example-1)
        * [Monitor](#defaults---monitor)
        * [Slack](#defaults---slack)
        * [WebHook](#defaults---webhook)
      - [Services](#services)
        * [Example](#example-2)
        * [Monitor](#services---monitor)
        * [Slack](#services---slack)
        * [WebHook](#services---webhook)

## Output
```bash
$ release_notifier -config myConfig.yml
2021/06/08 10:34:53 INFO: 5 sites to monitor:
2021/06/08 10:34:53 INFO: ansible/awx-operator, go-gitea/gitea, grafana/grafana, mattermost/mattermost-server, prometheus/prometheus
2021/06/08 10:34:54 INFO: ansible/awx-operator, Starting Release - 0.10.0
2021/06/08 10:36:27 INFO: go-gitea/gitea, Starting Release - v1.14.2
2021/06/08 10:36:42 INFO: grafana/grafana, Starting Release - v8.0.0
2021/06/08 10:36:52 INFO: mattermost/mattermost-server, Starting Release - 5.35.2
2021/06/08 10:37:08 INFO: prometheus/prometheus, Starting Release - v2.27.1
2021/06/10 16:13:49 INFO: grafana/grafana, New Release - v8.0.1
2021/06/10 16:13:49 INFO: Grafana, Slack message sent
2021/06/10 16:13:50 INFO: Grafana, WebHook received (202)
```

## Command-line arguments
```bash
$ release_notifier -h
Usage of /usr/local/bin/release_notifier:
  -config string
        The path to the config file to use (default "config.yml")
  -verbose
        Toggle verbose logging
```

## Config formatting
#### Example
```yaml
defaults:
  monitor:
    interval: 600
    access_token: 'GITHUB_ACCESS_TOKEN'
    allow_invalid: false
    ignore_misses: false
  slack:
    message: '<${monitor_url}|${monitor_id}> - ${version} released'
    username: 'Release Notifier'
    icon_url: 'https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png'
    maxtries: 3
  webhook:
    desired_status_code: 0
    maxtries: 3
    silent_fails: false
services:
  - id: CV-Site
    monitor:
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
        interval: 300
    webhook:
      type: github
      url: https://AWX_HOSTNAME/api/v2/job_templates/35/github/
      secret: SECRET_KEY
    slack:
      url: https://SLACK_INCOMING_WEBHOOK
```
Above, I set defaults.monitor.interval to 600 and then don't define an interval for the golang/go monitor. Therefore, this monitor will follow defaults.monitor.interval and query the site every 600 seconds. But, I can override that interval by stating it, for example in the adnanh/webhook monitor, I have set interval to 300 and so that page will be queried every 300 seconds.

#### Defaults
Defaults are not needed in the config, but you can override the coded defaults with your own defaults for Monitors, WebHooks and Slack messages in this defaults section. You could for example have a tiny defaults section that only has `defaults -> slack -> username: 'USERNAME'`, you do not need to define all values for a section. In the examples below, the values set are the coded defaults that will be used if they haven't been included in the service being monitored or the config defaults. (excluding access_token)

##### Example
```yaml
defaults:
  monitor:
    interval: 600
    access_token: 'GITHUB_ACCESS_TOKEN'
    allow_invalid: false
  slack:
    message: '<${monitor_url}|${monitor_id}> - ${version} released'
    username: 'Release Notifier'
    icon_emoji: ':github:'
    icon_url: ''
    maxtries: 3
  webhook:
    desired_status_code: 0
    maxtries: 3
    silent_fails: false
```

##### Defaults - Monitor
```yaml
defaults:
  monitor:
    interval: 600                       # Time between monitor queries.
    access_token: 'GITHUB_ACCESS_TOKEN' # Increase API rate limit with an access token (and allow querying private repos). Used when type="github".
    allow_invalid: false                # Allow invalid HTTPS Certificates.
    ignore_misses: false                # Ignore url_command fails (e.g. split on text that doesn't exist)
```

##### Defaults - Slack
```yaml
defaults:
  slack:
    message: '<${monitor_url}|${monitor_id}> - ${version} released' # Formatting of the message to send.
    username: 'Release Notifier'                                    # The user to message as.
    icon_emoji: ':github:'                                          # The emoji icon for that user.
    icon_url: ''                                                    # The URL of an icon for that user.
    maxtries: 3                                                     # Number of times to resend until a 2XX status code is received.
```
message - Each element of the monitor array can trigger a Slack message. This is the message that is sent when a change in version is noticed.
- `${monitor}` will be replaced with the id given to the monitor element that has changed version and that text will link to the url of that monitor element.
- `${version}` will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${service}` will be replaced with the id of the monitor element triggering the message.

##### Defaults - WebHook
```yaml
defaults:
  webhook:
    desired_status_code: 0 # Status code indicating a success. Send maxtries # of times until we receive this. 0 = accept any 2XX code.
    maxtries: 3            # Number of times to resend until desired_status_code is received.
    silent_fails: false    # Whether to notify Slack if a webhook fails maxtries times
```

#### Services
##### Example
```yaml
services:
  - id: CV-Site
    monitor:
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
        interval: 345
    slack:
      - url: https://SLACK_INCOMING_WEBHOOK
      - url: https://OTHER_SLACK_INCOMING_WEBHOOK
    webhook:
      - type: github
        url: https://AWX_HOSTNAME/api/v2/job_templates/35/github/
        secret: SECRET_KEY
      - type: github
        url: https://OTHER_AWX_HOSTNAME/api/v2/job_templates/7/github/
        secret: OTHER_SECRET_KEY
  - id: Gitea
    monitor:
      type: github
      url: go-gitea/gitea
    webhook:
      type: github
      url: https://AWX_HOSTNAME/api/v2/job_templates/36/github/
      secret: SECRET
```
This program can monitor multiple services. Just provide them in the standard yaml list format like above. It can also monitor multiple sites (if the same style Slack message(s) and webhook(s) are wanted), and can send multiple Slack messages and/or github style webhooks when a new release is found. Just turn the monitor, webhook and Slack sections into yaml lists. Note, if you don't have multiple Slack messages, multiple webhooks or multiple sites/repos under the same service, you don't have to format it as a list. This can be seen in the Gitea example above (they don't all need to be lists. e.g. you could have a list of monitors with a single webhook that isn't formatted as a list).

##### Services - Monitor
```yaml
services:
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${service} in Slack messages.
    monitor:                  # Required.
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
      allow_invalid: false                             # Optional. Allow invalid HTTPS Certificates.
      access_token: 'GITHUB_ACCESS_TOKEN'              # Optional. GitHub access token to use. Allows smaller interval (higher API rate limit).
      skip_slack: false                                # Optional. Don't send Slack messages for new releases of this monitor.
      skip_webhook: false                              # Optional. Don't send WebHooks for new releases of this monitor.
      interval: 600                                    # Optional. Amount of seconds to sleep between querying the URL for the version.
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

##### Services - Slack
```yaml
services:
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${service} in Slack messages.
    monitor:                  # Required.
      ....
    slack:                    # Optional.
      url: "SLACK_INCOMING_WEBHOOK"                                   # Required. The URL of the incoming Slack WebHook to send the message to.
      message: '<${monitor_url}|${monitor_id}> - ${version} released' # Optional. Formatting of the message to send.
      username: 'Release Notifier'                                    # Optional. The user to message as.
      icon_emoji: ':github:'                                          # Optional. The emoji icon for that user.
      icon_url: ''                                                    # Optional. The URL of an icon for that user.
      maxtries: 3                                                     # Optional. The number of times to resend until a 2XX status code is received.
```
The values of the optional arguments are the default values.

message:
- `${monitor}` will be replaced with the id given to the monitor element that has changed version and that text will link to the url of that monitor element.
- `${version}` will be replaced with the version that was found (e.g. `${version} = 10.6.3`).
- `${service}` will be replaced with the id of the monitor element triggering the message.

(of the monitor element that is triggering the message)

##### Services - WebHook
```yaml
  - id: "PRETTY_SERVICE_NAME" # Optional. Replaces ${service} in Slack messages.
    monitor:                  # Required.
      ...
    webhook:                  # Optional.
      type: "github"         # Required. The type of WebHook to send (Currently only github is supported).
      url: "WEBHOOK_URL"     # Required. The URL to send the WebHook to.
      secret: "SECRET"       # Required. The secret to send the WebHook with.
      desired_status_code: 0 # Optional. Keep sending the WebHooks until we recieve this status code (0 = accept any 2XX code).
      maxtries: 3            # Optional. Number of times to try re-sending WebHooks until we receive desired_status_code
      silent_fails: false    # Optional. Whether to send Slack messages to the Slacks of the Monitor when a WebHook fails maxtries times.
```
The values of the optional arguments are the default values.
