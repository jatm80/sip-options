# Define the environment variables
export SIP_DEST_IP = sip.server.com
export SLACK_CHANNEL = #test_alerts
export SLACK_USER = spam_bot
export environment = Dev
export SLACK_WEBHOOK_URL = https://hooks.slack.com/services/abc/jdaslkdjasldjasldkajsld

# Ensure that all variables are exported
export

# Define the build target
.PHONY: build test

build:
	@echo "Building the project for Linux AMD64..."
	@env GOOS=linux GOARCH=amd64 go build -o bin/sip_options .
	@echo "Build completed successfully."

# Define the test target
test:
	@echo "Running go tests with the following environment variables:"
	@echo "SIP_DEST_IP=$(SIP_DEST_IP)"
	@echo "SLACK_CHANNEL=$(SLACK_CHANNEL)"
	@echo "SLACK_USER=$(SLACK_USER)"
	@echo "environment=$(environment)"
	@echo "SLACK_WEBHOOK_URL=$(SLACK_WEBHOOK_URL)"
	@go test
	@echo "Tests completed successfully."

# Default target when running `make` without arguments
.PHONY: all
all: build test
	@echo "Build and tests completed successfully."
