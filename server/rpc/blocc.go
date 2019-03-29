package rpc

import (
	"encoding/json"
)

// MarshalBinary used to store in cache
func (mps *MemPoolStats) MarshalBinary() (data []byte, err error) {
	return json.Marshal(mps)
}

// UnmarshalBinary is used to rtrieve from cache
func (mps *MemPoolStats) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, mps)
}
