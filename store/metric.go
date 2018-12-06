package store

type Metric struct {
	Type    string             `json:"type"`
	Symbol  string             `json:"symbol"`
	BlockId string             `json:"blockId"`
	Time    int64              `json:"time"`
	TxId    string             `json:"txId"`
	Tags    map[string]string  `json:"tag"`
	Metrics map[string]float64 `json:"metric"`
}
