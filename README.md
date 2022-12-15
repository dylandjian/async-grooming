# Groom-bot

This bot was made initially to learn Go and to find a cool usecase. I have found
that it was an interesting concept to be able to groom tickets asynchronously
through Slack. In order to that, we had to track tickets without having to
scroll through the Slack channel.

## Purpose

The goal of this Bot is to be able to easily have a recap of the tickets that we
are currently talking about and their status. We can also vote on tickets which
will transfer the tickets from a status of "grooming" to "groomed".

## How does it work ?

It works by using the Slack API to fetch the messages in the grooming channel.
After all the messages have been fetched (from the previous 14 days), they are
being compared to a local database that is stored in a csv file. Each time a
ticket is groomed (after having 3 ✅ reaction emoji in our case but that can be
configured) it is added to the csv file, therefore allowing the tickets that
have already been talked about to be removed.

There is also an admin feature which allows bypassing the emoji validation (that
can be configured as well).

Then, once all the tickets and their respective status have been found a message
template is created using the [Block Kit](https://api.slack.com/block-kit)
developed by Slack.

### Template

You can customize the template by modifying the file `block-template.json` which
is a JSON export of the [API Block Kit builder](https://app.slack.com/block-kit-builder)
on which you can test your specific message.

## Installation

If you want to run the groom-bot you will need :

- Go >= 1.14 (hasn't been tested below, only if you want to rebuild the bot)
- MacOS (hasn't been tested on anything else)

You are going to need to find the following informations :

- The slack channel id of the channel you want the bot to post in
- A valid slack token for the bot so that it can contacts the API
- A slack domain URL to be able to query from your Slack

### Usage

This is how you call the bot

```sh
./grooming-bot --groomingChannelId YOUR_GROOMING_CHANNEL_ID --token
YOUR_SLACK_TOKEN --slackDomain YOUR_SLACK_DOMAIN
```

Here is the complete usage of the bot that you can have by doing `./grooming-bot
--help`

```mrkdwn
-debug
      Show JSON output (default false)
-emojiValidationName string
      Emoji name to verify the number of validations (default "white_check_mark")
-emojiAdmin string
      Emoji to bypass the team size approval (default "ok")
-firstInitBot string
      First date at which to start fetching messages from
-groomingChannelId string
      SlackID of the grooming channel
-slackDomain string
      Slack domain in which to query
-teamSizeApproval int
      Number of approval required to move a ticket (default 3)
-token string
      Slack Token
```

You also probably want to setup the grooming bot to run in a cronjob.
How you do that you MacOS is using the following method :

```sh
crontab -e
```

will open the crontab edition menu.
How it works is that for each line in the file it is going to create a cronjob
at the specified times. I recommand using [crontab guru](https://crontab.guru/)
in order to find the pattern that you want. Then on the right side of the timer,
the command that needs to be executed. I recommand making a small script which
exports the PATH and execute the bot. Something along these lines :

```sh
#!/bin/sh

PATH=YOUR_PATH
cd ~/PATH/TO/BOT && ./grooming-bot
```

## Improvements

Some improvements that are already planned are :

- Moving the tickets on the actual JIRA board once we consider them ready to be sized.
- Add a slack command to be able to generate a ticket (would add more structure
  to the way we add slack messages in JIRA ticket descriptions and also in the
  slack channel)
