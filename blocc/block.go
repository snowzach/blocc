package blocc

import (
	"encoding/base64"

	"github.com/gogo/protobuf/proto"
)

const (
	TypeBlock  = "block"
	TypeTx     = "tx"
	TypeMetric = "metric"
)

type Raw []byte

func (r Raw) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(r) + `"`), nil
}

func (r Raw) UnmarshalJSON(in []byte) error {
	// Remove the beginning and ending "
	r = make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	_, err := base64.StdEncoding.Decode(r, in)
	return err
}

func (b *Block) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(b)
}

func (b *Block) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, b)
}

func (tx *Tx) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(tx)
}

func (tx *Tx) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, tx)
}
