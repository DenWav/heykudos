heykudos
========

HeyKudos is a Slack bot to give other people in your Slack organization "kudos" by sending emojis to each other.
This is done by pinging a user with `@` and including an emoji (including custom emojis) in the message as well.
This message needs to be done in an enabled channel, and channels can be enabled with `@heykudos enable`.

The people with the most kudos can be viewed with the leaderboard with `@heykudos leaderboard`. Leaderboards for individual
sets of emojis can be viewed as well with `@heykudos leaderboard <emoji1> <emoji2>...`.

Requirements
------------

HeyKudos requires 2 dependencies: [Go](https://golang.org/) and [MySQL](https://www.mysql.com/). The MySQL database can be
setup following the instructions below.

Configuration
-------------

MySQL is required. To set up the database, run the following commands:

```bash
mysql -u root < sql/create.sql
```

> **Warning**:
>
> This will create a user called `kudos` and a database called `kudos`. It will completely delete any existing users or
databases called `kudos` first. If you already have a database or user going by that name, change the `create.sql`
script accordingly first before running it.

Now in the same directory as the `heykudos` executable, should be a `config.json` file:

```json
{
  "botToken": "<Bot User OAuth Access Token>",
  "userToken": "<OAuth Access Token>",
  "db": {
    "database": "kudos",
    "username": "kudos",
    "password": "kudos",
    "hostname": "localhost",
    "port": 3306
  },
  "amountPerDay": 5
}
```

Change any database configuration as necessary based on the database setup.

`botToken` represents the Slack Bot OAuth Access Token which can be found on the `OAuth & Permissions` page of the Slack
app configuration. `userToken` represents the standard Slack OAuth Access Token, which can be found on the same page.

Both tokens are required, as they are used for different APIs.

`amountPerDay` represents the total number of kudos any given user is allowed to give out per day. It will reset at
midnight of the local system time (based on the SQL date returned by `DATE(NOW())`).

Running
-------

`heykudos` takes no arguments, only the configuration file is used as input. The configuration file must be called
`config.json` in the current working directory when `heykudos` is called.

The following `systemd` config is the recommended method of running `heykudos`:

```
[Unit]
Description=heykudos
After=syslog.target network.target mysql.target

[Service]
Type=simple
Restart=always
RestartSec=1
User=kudos
ExecStart=<executable location>
WorkingDirectory=<working directory with config.json>
StandardOutput=syslog
StandardError=syslog

[Install]
WantedBy=multi-user.target
```

Fill in the `ExecStart` and `WorkingDirectory` fields, and place that in a new file
`/etc/systemd/system/heykudos.service`. The user can be whatever user you want, `kudos` isn't required, but it's
recommended not to run anything as root.

Now you can start the service:

```bash
systemctl start heykudos
```

And to automatically start the service on reboot, enable it:

```bash
systemctl enable heykudos
```
