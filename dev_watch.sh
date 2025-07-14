#!/usr/bin/env bash

> debug.log

while true; do
    go build -o _build/clai ./cmd/clai
    ./_build/clai &
    APP_PID=$!

    # Wait for a file change
    inotifywait -r -e modify,close_write --exclude '(^|/)(\.git|debug\.log)$' . > /dev/null

    # Kill the running app
    kill $APP_PID 2>/dev/null
    wait $APP_PID 2>/dev/null
done
