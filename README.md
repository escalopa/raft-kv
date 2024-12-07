# raft-kv 🗃️

A **raft** based **key-value** store.

![UI](./static/demo.png)

## Prerequisites 📚

- [task](https://taskfile.dev/#/installation)
- [docker-compose](https://docs.docker.com/compose/install/)
- [catalystgo](https://github.com/catalystgo/cli)

## Run 🚀

Start the raft cluster

```bash
task docker-up
```

Start the UI

```bash
task ui
```

### Endpoint 🧭

| App       | Endpoint              |
|-----------|-----------------------|
| UI        | <http://localhost:3000> |
| Dozzle    | <http://localhost:8080> |
| Portainer | <http://localhost:9000> |

## API 📖

- [raft](./api/raft/raft.proto)
- [kv](./api/kv/kv.proto)

## Config ⚙️

TODO
