package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	avaJson "github.com/ava-labs/avalanchego/utils/json"
	"github.com/ybbus/jsonrpc/v3"
)

const (
	ClientRequestTimeout = 10 * time.Second
)

// Copy-paste from
//
//	"github.com/ava-labs/avalanchego/vms/platformvm/service"
//
// To avoid an additional dependency
type GetRewardUTXOsReply struct {
	NumFetched avaJson.Uint64      `json:"numFetched"`
	UTXOs      []string            `json:"utxos"`
	Encoding   formatting.Encoding `json:"encoding"`
}

type RPCClient interface {
	GetRewardUTXOs(id ids.ID) (*GetRewardUTXOsReply, error)
	GetTx(id ids.ID) (*api.GetTxReply, error)
}

type AvalancheRPCClient struct {
	client jsonrpc.RPCClient
}

func NewAvalancheRPCClient(endpoint string) *AvalancheRPCClient {
	return &AvalancheRPCClient{
		client: jsonrpc.NewClient(endpoint),
	}
}

func (c *AvalancheRPCClient) GetRewardUTXOs(id ids.ID) (*GetRewardUTXOsReply, error) {
	params := api.GetTxArgs{
		TxID:     id,
		Encoding: formatting.Hex,
	}
	ctx, cancelCtx := context.WithTimeout(context.Background(), ClientRequestTimeout)
	defer cancelCtx()

	response, err := c.client.Call(ctx, "platform.getRewardUTXOs", params)
	if err != nil {
		return nil, err
	}

	reply := &GetRewardUTXOsReply{}
	err = response.GetObject(reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (c *AvalancheRPCClient) GetTx(id ids.ID) (*api.GetTxReply, error) {
	params := api.GetTxArgs{
		TxID:     id,
		Encoding: formatting.Hex,
	}
	ctx, cancelCtx := context.WithTimeout(context.Background(), ClientRequestTimeout)
	defer cancelCtx()

	response, err := c.client.Call(ctx, "platform.getTx", params)
	if err != nil {
		return nil, err
	}

	reply := &api.GetTxReply{}
	err = response.GetObject(reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

//
// Implement RPCClient interface using recorded data
//

type RPCRecording struct {
	Id    string          `json:"id"`
	UTXOs []string        `json:"utxos"`
	Tx    json.RawMessage `json:"tx"`
}

func (r *RPCRecording) toGetRewardUTXOsReply() *GetRewardUTXOsReply {
	return &GetRewardUTXOsReply{
		NumFetched: avaJson.Uint64(len(r.UTXOs)),
		UTXOs:      r.UTXOs,
		Encoding:   formatting.Hex,
	}
}

func (r *RPCRecording) toGetTxReply() *api.GetTxReply {
	return &api.GetTxReply{
		Tx:       r.Tx,
		Encoding: formatting.Hex,
	}
}

type RecordedRPCClient struct {
	txIDToRecording map[string]*RPCRecording
}

func NewRecordedRPCClient(fileName string) (*RecordedRPCClient, error) {
	recordings, err := readUTXORecordings(fileName)
	if err != nil {
		return nil, err
	}
	txIDToRecording := make(map[string]*RPCRecording)
	for _, recording := range recordings {
		txIDToRecording[recording.Id] = recording
	}
	return &RecordedRPCClient{txIDToRecording: txIDToRecording}, nil
}

func (c *RecordedRPCClient) GetRewardUTXOs(id ids.ID) (*GetRewardUTXOsReply, error) {
	if reply, ok := c.txIDToRecording[id.String()]; ok {
		return reply.toGetRewardUTXOsReply(), nil
	}
	return nil, fmt.Errorf("no recording for tx %v", id)
}

func (c *RecordedRPCClient) GetTx(id ids.ID) (*api.GetTxReply, error) {
	if reply, ok := c.txIDToRecording[id.String()]; ok {
		return reply.toGetTxReply(), nil
	}
	return nil, fmt.Errorf("no recording for tx %v", id)
}

func readUTXORecordings(fileName string) ([]*RPCRecording, error) {
	var recordings []*RPCRecording
	jsonFile, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonFile), &recordings)
	if err != nil {
		return nil, err
	}
	return recordings, nil
}
