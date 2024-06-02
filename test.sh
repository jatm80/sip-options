#!/bin/bash

export SIP_DEST_IP="sip.server.com"
export SLACK_CHANNEL="#test_alerts"
export SLACK_USER="spam_bot"
export environment="Dev"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/abc/jdaslkdjasldjasldkajsld"
go test

