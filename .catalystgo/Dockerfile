FROM golang:alpine AS builder
WORKDIR /go/src/github.com/escalopa/raft-kv
COPY . .
RUN go mod download
RUN go build -o /go/bin/raft-kv ./cmd/.

FROM alpine:3.19 AS production
COPY --from=builder /go/bin/raft-kv /go/bin/raft-kv
COPY /go/bin/raft-kv /go/bin/raft-kv
COPY ./.cataystgo/ /go/bin/.catalystgo/
ENTRYPOINT ["/go/bin/raft-kv"]
