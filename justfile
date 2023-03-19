gateway:
    go run . gateway --token $VPBOT_TOKEN --dev-logging

showcase:
    go run . showcase --channel-id $VPBOT_SHOWCASE_CHANNEL --dev-logging
