#!/bin/bash

# exit on error
set -e

# generate random key-value pairs
generate_random_pair() {
  local key="key$(openssl rand -hex 4)"
  local value="value$(openssl rand -hex 4)"
  echo "{\"key\": \"$key\", \"value\": \"$value\"}"
}

# find the leader
find_leader() {
  local servers=(${1//,/ }) # split server list into array
  for server in "${servers[@]}"; do
    state=$(curl -s "$server/raft/info" | jq -r '.state')
    if [ "$state" == "leader" ]; then
      echo "$server"
      return
    fi
  done
}

# ensure at least two arguments are provided
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <N> <server_list>"
  exit 1
fi

N=$1
SERVER_LIST=$2

# find the leader server
LEADER=$(find_leader "$SERVER_LIST")

if [ -z "$LEADER" ]; then
  echo "leader not found. Exiting."
  exit 1
fi

echo "leader found: $LEADER"

# send N messages to the leader
for ((i = 1; i <= N; i++)); do
  PAIR=$(generate_random_pair)
  RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$LEADER/kv" -H "Content-Type: application/json" -d "$PAIR")
  if [ "$RESPONSE" -ne 200 ]; then
    echo "error occurred while sending message $i. HTTP Status: $RESPONSE"
    exit 1
  fi
  echo -ne "\rsend $i request"
done

echo -ne "\ndone\n"