package blocc

import (
	"bytes"
	"encoding/base64"

	"github.com/gogo/protobuf/proto"
)

// These are constants used for identifying blocc types
const (
	TypeBlock = "block"
	TypeTx    = "tx"
)

// Blocc raw fields are byte arrays
type Raw []byte

// MarshalJSON for Raw fields is represented as base64
func (r *Raw) MarshalJSON() ([]byte, error) {
	if r == nil || *r == nil || len(*r) == 0 {
		return []byte(`""`), nil
	}
	return []byte(`"` + base64.StdEncoding.EncodeToString(*r) + `"`), nil
}

// UnmarshalJSON for Raw fields is parsed as base64
func (r *Raw) UnmarshalJSON(in []byte) error {
	var ret *[]byte
	if in == nil || len(in) == 0 {
		*ret = []byte{}
		return nil
	}
	// Remove the beginning and ending "
	in = bytes.Trim(in, `"`)
	*r = make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	_, err := base64.StdEncoding.Decode(*r, in)
	return err
}

// Used to fulfill the BinaryMarshaler interface from encoding
func (b *Block) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(b)
}

// Used to fulfill the BinaryUnmarshaler interface from encoding
func (b *Block) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, b)
}

// Used to fulfill the BinaryMarshaler interface from encoding
func (tx *Tx) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(tx)
}

// Used to fulfill the BinaryUnmarshaler interface from encoding
func (tx *Tx) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, tx)
}

// Used to fulfill the BinaryMarshaler interface from encoding
func (bh *BlockHeader) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(bh)
	// return json.Marshal(bh)
}

// Used to fulfill the BinaryUnmarshaler interface from encoding
func (bh *BlockHeader) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, bh)
	// return json.Unmarshal(data, bh)
}

// BlockHeader returns just the header from a block
func (b *Block) BlockHeader() *BlockHeader {
	if b == nil {
		return nil
	}
	return &BlockHeader{
		BlockId:     b.BlockId,
		Height:      b.Height,
		PrevBlockId: b.PrevBlockId,
		Time:        b.Time,
	}
}

// GetHeightSafe returns the block height safely if any part is nil (returning blocc.HeightUnknown)
func (b *Block) GetHeightSafe() int64 {
	if b == nil {
		return HeightUnknown
	}
	return b.Height
}

// GetHeightSafe returns the block height safely if any part is nil (returning blocc.HeightUnknown)
func (bh *BlockHeader) GetHeightSafe() int64 {
	if bh == nil {
		return HeightUnknown
	}
	return bh.Height
}

// DataValue fetches a data value from a block retuning a blank string if not found
func (b *Block) DataValue(key string) string {
	if b == nil || b.Data == nil {
		return ""
	}
	if value, ok := b.Data[key]; ok {
		return value
	}
	return ""
}

// DataValue fetches a data value from a tx retuning a blank string if not found
func (tx *Tx) DataValue(key string) string {
	if tx == nil || tx.Data == nil {
		return ""
	}
	if value, ok := tx.Data[key]; ok {
		return value
	}
	return ""
}

// DataValue fetches a data value from a txin retuning a blank string if not found
func (txin *TxIn) DataValue(key string) string {
	if txin == nil || txin.Data == nil {
		return ""
	}
	if value, ok := txin.Data[key]; ok {
		return value
	}
	return ""
}

// DataValue fetches a data value from a txout retuning a blank string if not found
func (txout *TxOut) DataValue(key string) string {
	if txout == nil || txout.Data == nil {
		return ""
	}
	if value, ok := txout.Data[key]; ok {
		return value
	}
	return ""
}

/* Need to figue out why protobuf is still generating these with goproto_stringer = false
func (bh *BlockHeader) String() string {
	if bh == nil {
		return "null"
	}
	return fmt.Sprintf(`{"block_id":%q,"height":%q,"prev_block_id":%s}`, bh.BlockId, bh.Height, bh.PrevBlockId)
}

func (b *Block) String() string {
	if b == nil {
		return "null"
	}
	return fmt.Sprintf(`{"block_id":%q,"height":%q,"prev_block_id":%s}`, b.BlockId, b.Height, b.PrevBlockId)
}
*/
