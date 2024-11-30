package core

import (
	"encoding/binary"
	"encoding/json"
)

type Command string

func (c Command) String() string {
	return string(c)
}

const (
	Set Command = "SET"
	Del Command = "DEL"
)

type Entry struct {
	// Term is the term of the log entry
	Term uint64 `json:"term"`

	// Index is the index of the log entry
	Index uint64 `json:"index"`

	// Data is the command of the log entry as an array of strings
	// Example: ["SET", "key", "value"], ["DEL", "key"]
	Data []string `json:"data"`
}

func (e *Entry) ToBytes() ([]byte, error) {
	return json.Marshal(e)
}

func EntryFromBytes(bytes []byte) (*Entry, error) {
	var e Entry

	err := json.Unmarshal(bytes, &e)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func UintToKey(index uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, index)
	return b
}

func KeyToUint(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
