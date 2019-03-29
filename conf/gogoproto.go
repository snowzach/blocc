package conf

import (
	"github.com/gogo/protobuf/version"
)

// GetGoGoProtoVersion forces gogoproto to be installed as a dependancy to this program
// so we can use it's includes when building the protobufs
func GetGoGoProtoVersion() string {
	return version.Get()
}
