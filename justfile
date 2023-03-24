common-args := "--dev-logging"

default:
  just --list

gateway:
    go run . gateway --token $VPBOT_TOKEN {{common-args}}

showcase:
    go run . showcase --channel-id $VPBOT_SHOWCASE_CHANNEL {{common-args}}

github:
    go run . github --channel-id $VPBOT_GITHUB_CHANNEL --role-id $VPBOT_GITHUB_MENTION_ROLE {{common-args}}
