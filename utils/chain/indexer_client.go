package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/indexer"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/rpc"
)

type IndexerClient interface {
	GetContainerRange(ctx context.Context, from uint64, numToFetch int) ([]indexer.Container, error)
	GetLastAccepted(ctx context.Context) (indexer.Container, uint64, error)
	GetContainerByIndex(ctx context.Context, index uint64) (indexer.Container, error)
	GetIndex(ctx context.Context, id ids.ID) (uint64, error)
}

// Implement IndexerClientBase using Avalanche indexer
type AvalancheIndexerClient struct {
	client     *indexer.Client
	rpcOptions []rpc.Option
}

func NewAvalancheIndexerClient(uri string, opts ...rpc.Option) *AvalancheIndexerClient {
	client := indexer.NewClient(uri)
	return &AvalancheIndexerClient{client: client, rpcOptions: opts}
}

func (ic *AvalancheIndexerClient) GetLastAccepted(ctx context.Context) (indexer.Container, uint64, error) {
	return ic.client.GetLastAccepted(ctx, ic.rpcOptions...)
}

func (ic *AvalancheIndexerClient) GetContainerByIndex(ctx context.Context, index uint64) (indexer.Container, error) {
	return ic.client.GetContainerByIndex(ctx, index, ic.rpcOptions...)
}

func (ic *AvalancheIndexerClient) GetContainerRange(ctx context.Context, from uint64, numToFetch int) ([]indexer.Container, error) {
	return ic.client.GetContainerRange(ctx, from, numToFetch, ic.rpcOptions...)
}

func (ic *AvalancheIndexerClient) GetIndex(ctx context.Context, id ids.ID) (uint64, error) {
	return ic.client.GetIndex(ctx, id, ic.rpcOptions...)
}

//
// Implement IndexerClientBase indexer using recorded data
//

// Item returned by method index.getContainerByIndex
type ContainerRecording struct {
	Id        string    `json:"id"`
	Bytes     string    `json:"bytes"`
	Timestamp time.Time `json:"timestamp"`
	Index     string    `json:"index"`
}

func (r *ContainerRecording) toContainer() (*indexer.Container, uint64, error) {
	id, err := ids.FromString(r.Id)
	if err != nil {
		return nil, 0, err
	}

	index, err := strconv.ParseUint(r.Index, 10, 64)
	if err != nil {
		return nil, 0, err
	}

	bytes, err := formatting.Decode(formatting.Hex, r.Bytes)
	if err != nil {
		return nil, 0, err
	}

	result := indexer.Container{
		ID:        id,
		Bytes:     bytes,
		Timestamp: r.Timestamp.UnixNano(),
	}
	return &result, index, nil
}

func readContainerRecordings(fileName string) ([]ContainerRecording, error) {
	var recordings []ContainerRecording
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

type RecordedIndexerClient struct {
	idxToContainer map[uint64]*indexer.Container
	idToIndex      map[ids.ID]uint64
	maxIndex       uint64
}

func NewRecordedIndexerClient(fileName string) (*RecordedIndexerClient, error) {
	rec, err := readContainerRecordings(fileName)
	if err != nil {
		return nil, err
	}

	idxToContainer := make(map[uint64]*indexer.Container)
	idToIndex := make(map[ids.ID]uint64)
	maxIndex := uint64(0)
	for _, r := range rec {
		c, index, err := r.toContainer()
		if err != nil {
			return nil, err
		}
		idxToContainer[index] = c
		idToIndex[c.ID] = index
		if index > maxIndex {
			maxIndex = index
		}
	}
	return &RecordedIndexerClient{idxToContainer, idToIndex, maxIndex}, nil
}

func (ic *RecordedIndexerClient) GetLastAccepted(ctx context.Context) (indexer.Container, uint64, error) {
	return *ic.idxToContainer[ic.maxIndex], ic.maxIndex, nil
}

func (ic *RecordedIndexerClient) GetContainerByIndex(ctx context.Context, index uint64) (indexer.Container, error) {
	if c, ok := ic.idxToContainer[index]; ok {
		return *c, nil
	}
	return indexer.Container{}, fmt.Errorf("container with index %d not found", index)
}

func (ic *RecordedIndexerClient) GetContainerRange(ctx context.Context, from uint64, numToFetch int) ([]indexer.Container, error) {
	result := make([]indexer.Container, 0, numToFetch)
	i := from

	if c, ok := ic.idxToContainer[i]; ok {
		result = append(result, *c)
		i++
	} else {
		return nil, fmt.Errorf("invalid from value %d", from)
	}

	to := from + uint64(numToFetch)
	for ; i < to; i++ {
		if c, ok := ic.idxToContainer[i]; ok {
			result = append(result, *c)
		} else {
			break
		}
	}
	return result, nil
}

func (ic *RecordedIndexerClient) GetIndex(ctx context.Context, id ids.ID) (uint64, error) {
	if index, ok := ic.idToIndex[id]; ok {
		return index, nil
	}
	return 0, fmt.Errorf("container with id %v not found", id)
}
