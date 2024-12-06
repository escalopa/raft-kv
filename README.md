# raft-kv 🗃️

A **raft** based **key-value** store.

![UI](./static/demo.png)

## Prerequisites

- [task](https://taskfile.dev/#/installation)
- [docker-compose](https://docs.docker.com/compose/install/)
- [catalystgo](https://github.com/catalystgo/cli)

## Run 

Start the raft cluster

```bash
task docker-up
```

Start the UI

```bash
task ui
```

### Endpoint

| App       | Endpoint              |
|-----------|-----------------------|
| UI        | http://localhost:3000 |
| Dozzle    | http://localhost:8080 |
| Portainer | http://localhost:9000 |

## API

- [raft](./api/raft/raft.proto)
- [kv](./api/kv/kv.proto)

## Config

| ENV                            | Description                                      | Default Value | Example                           |
|--------------------------------|--------------------------------------------------|---------------|-----------------------------------|
| LOG_LEVEL                      | Logging level for the application                | WARN          | INFO                              |
| RAFT_ID                        | Node identifier                                  |               | 1                                 |
| RAFT_CLUSTER                   | Cluster node addresses (format: ID@HOST:PORT)    |               | 1@raft-kv-1:8000,2@raft-kv-2:8000 |
| RAFT_COMMIT_PERIOD             | Commit period (ms)                               | 50            | 100                               |
| RAFT_APPEND_ENTRIES_TIMEOUT    | Append entries timeout (ms)                      | 150           | 200                               |
| RAFT_REQUEST_VOTE_TIMEOUT      | Request vote timeout (ms)                        | 150           | 200                               |
| RAFT_ELECTION_DELAY_PERIOD     | Election delay period (ms)                       | 3000          | 4000                              |
| RAFT_ELECTION_TIMEOUT_PERIOD   | Election timeout (ms)                            | 300           | 400                               |
| RAFT_HEARTBEAT_PERIOD          | Heartbeat interval (ms)                          | 50            | 100                               |
| RAFT_LEADER_STALE_PERIOD       | Leader stale threshold (ms)                      | 200           | 300                               |
| RAFT_LEADER_STALE_CHECK_PERIOD | Leader stale check frequency (ms)                | 100           | 150                               |
| BADGER_ENTRY_PATH              | Log storage path                                 | /data/entry   | /var/entry                        |
| BADGER_STATE_PATH              | Raft state storage path                          | /data/state   | /var/state                        |
| BADGER_KV_PATH                 | State machine storage path                       | /data/kv      | /var/kv                           |
