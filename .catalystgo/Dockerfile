FROM golang:alpine3.20 AS dev
WORKDIR /go/src/github.com/escalopa/raft-kv
COPY . .
RUN go mod download
RUN mkdir -p /data/{entry,state,kv}
ENTRYPOINT ["go", "run", "./cmd/raft-kv/main.go", "--config", ".catalystgo/config-local.yml"]

FROM golang:alpine3.20 AS builder
WORKDIR /go/src/github.com/escalopa/raft-kv
COPY . .
RUN go mod download
RUN go build -o /go/bin/raft-kv ./cmd/main.go

FROM alpine:3.19 AS production
COPY --from=builder /go/bin/raft-kv /go/bin/raft-kv
COPY /go/bin/raft-kv /go/bin/raft-kv
COPY .cataystgo/ /go/bin/.catalystgo/
ENTRYPOINT ["/go/bin/raft-kv", "--config", ".catalystgo/config-local.yml"]
