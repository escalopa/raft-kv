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
 
[//]: # (## Config)

[//]: # (| Key                                | Description                                               |)

[//]: # (|------------------------------------|-----------------------------------------------------------|)

[//]: # (| RAFT_ID                            | ID of the node                                            |)

[//]: # (| RAFT_CLUSTER                       | Comma separated list of nodes                             |)

[//]: # (| RAFT_COMMIT_PERIOD                 | Period to commit the log &#40;if commit_index > last_applied&#41; |)

[//]: # (| RAFT_ELECTION_DELAY_PERIOD         | Period to delay the election &#40;only on startup&#41;            |)

[//]: # (| RAFT_ELECTION_TIMEOUT_PERIOD       | Period to timeout the election                            |)

[//]: # (| RAFT_HEARTBEAT_PERIOD              | Period to send the heartbeat                              |)

[//]: # (| RAFT_LEADER_STALE_PERIOD           | Period to check the leader staleness                      |)

[//]: # (| RAFT_LEADER_CHECK_STEP_DOWN_PERIOD | Period to check the leader step down                      |)

[//]: # (| BADGER_ENTRY_PATH                  | Path to store the entries                                 |)

[//]: # (| BADGER_STATE_PATH                  | Path to store the state                                   |)

[//]: # (| BADGER_KV_PATH                     | Path to store the key-value                               |)
