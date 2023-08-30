package cronjob

import (
	"context"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/utils"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/merkle"
	"math/big"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/ybbus/jsonrpc/v3"
)

var (
	merkleTreeItemABIObjectArguments abi.Arguments
)

func init() {
	bytes32Ty, err1 := abi.NewType("bytes32", "", nil)
	uint8Ty, err2 := abi.NewType("uint8", "", nil)
	bytes20Ty, err3 := abi.NewType("bytes20", "", nil)
	uint64Ty, err4 := abi.NewType("uint64", "", nil)
	err := utils.Join(err1, err2, err3, err4)
	if err != nil {
		panic(err)
	}
	merkleTreeItemABIObjectArguments = abi.Arguments{
		{
			Name: "txId",
			Type: bytes32Ty,
		},
		{
			Name: "stakingType",
			Type: uint8Ty,
		},
		{
			Name: "inputAddress",
			Type: bytes20Ty,
		},
		{
			Name: "nodeId",
			Type: bytes20Ty,
		},
		{
			Name: "startTime",
			Type: uint64Ty,
		},
		{
			Name: "endTime",
			Type: uint64Ty,
		},
		{
			Name: "weight",
			Type: uint64Ty,
		},
	}
}

type PermissionedValidators struct {
	Validators []*api.PermissionedValidator
}

func CallPChainGetConnectedValidators(client jsonrpc.RPCClient) ([]*api.PermissionedValidator, error) {
	ctx := context.Background()
	response, err := client.Call(ctx, "platform.getCurrentValidators")
	if err != nil {
		return nil, err
	}

	reply := PermissionedValidators{}
	err = response.GetObject(&reply)

	return reply.Validators, err
}

func TransactOptsFromPrivateKey(privateKey string, chainID int) (*bind.TransactOpts, error) {
	if privateKey[:2] == "0x" {
		privateKey = privateKey[2:]
	}

	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "crypto.HexToECDSA")
	}

	opts, err := bind.NewKeyedTransactorWithChainID(
		pk, big.NewInt(int64(chainID)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "bind.NewKeyedTransactorWithChainID")
	}
	// bind.N
	return opts, nil
}

type idAddressPair struct {
	TxID         string
	InputAddress string
}

// Deduplicate txs by (txID, address) pairs. This is necessary because the same tx can have
// multiple UTXO inputs.
func dedupeTxs(txs []database.PChainTxData) []database.PChainTxData {
	txSet := make(map[idAddressPair]*database.PChainTxData, len(txs))

	for i := range txs {
		tx := &txs[i]
		if tx.TxID == nil {
			continue
		}

		txSet[idAddressPair{*tx.TxID, tx.InputAddress}] = tx
	}

	dedupedTxs := make([]database.PChainTxData, 0, len(txSet))
	for _, tx := range txSet {
		dedupedTxs = append(dedupedTxs, *tx)
	}

	return dedupedTxs
}

func toStakeData(
	tx *database.PChainTxData,
) (*mirroring.IPChainStakeMirrorVerifierPChainStake, error) {
	txHash, err := ids.FromString(*tx.TxID)
	if err != nil {
		return nil, errors.Wrap(err, "ids.FromString")
	}

	txType, err := getTxType(tx.Type)
	if err != nil {
		return nil, err
	}

	nodeID, err := ids.NodeIDFromString(tx.NodeID)
	if err != nil {
		return nil, errors.Wrap(err, "ids.NodeIDFromString")
	}

	if tx.StartTime == nil {
		return nil, errors.New("tx.StartTime is nil")
	}

	startTime := uint64(tx.StartTime.Unix())

	if tx.EndTime == nil {
		return nil, errors.New("tx.EndTime is nil")
	}

	endTime := uint64(tx.EndTime.Unix())

	address, err := utils.ParseAddress(tx.InputAddress)
	if err != nil {
		return nil, errors.Wrap(err, "utils.ParseAddress")
	}

	return &mirroring.IPChainStakeMirrorVerifierPChainStake{
		TxId:         txHash,
		StakingType:  txType,
		InputAddress: address,
		NodeId:       nodeID,
		StartTime:    startTime,
		EndTime:      endTime,
		Weight:       tx.Weight,
	}, nil
}

func encodeTreeItem(tx *database.PChainTxData) ([]byte, error) {
	// ABI Encode mirroring.IPChainStakeMirrorVerifierPChainStake

	stakeData, err := toStakeData(tx)
	if err != nil {
		return nil, errors.Wrap(err, "toStakeData")
	}
	return merkleTreeItemABIObjectArguments.Pack(
		stakeData.TxId,
		stakeData.StakingType,
		stakeData.InputAddress,
		stakeData.NodeId,
		stakeData.StartTime,
		stakeData.EndTime,
		stakeData.Weight,
	)
}

func buildTree(txs []database.PChainTxData) (merkle.Tree, error) {
	hashes := make([]common.Hash, len(txs))

	for i := range txs {
		tx := &txs[i]

		if tx.TxID == nil {
			return merkle.Tree{}, errors.New("tx.TxID is nil")
		}

		encodedBytes, err := encodeTreeItem(tx)
		if err != nil {
			return merkle.Tree{}, errors.Wrap(err, "encodeTreeItem")
		}
		hashes[i] = crypto.Keccak256Hash(encodedBytes)
	}

	return merkle.Build(hashes, false), nil
}

func getMerkleRoot(votingData []database.PChainTxData) (common.Hash, error) {
	tree, err := buildTree(votingData)
	if err != nil {
		return [32]byte{}, err
	}
	return tree.Root()
}

type epochInfo struct {
	period        time.Duration
	periodSeconds int
	start         time.Time
}

func newEpochInfo(cfg *config.EpochConfig) epochInfo {
	return epochInfo{
		period:        cfg.Period,
		periodSeconds: int(cfg.Period.Seconds()),
		start:         cfg.Start,
	}
}

func (e epochInfo) getStartTime(epoch int64) time.Time {
	return e.start.Add(time.Duration(epoch) * e.period)
}

func (e epochInfo) getEndTime(epoch int64) time.Time {
	return e.getStartTime(epoch + 1)
}

func (e epochInfo) getTimeRange(epoch int64) (time.Time, time.Time) {
	return e.getStartTime(epoch), e.getEndTime(epoch)
}

func (e epochInfo) getEpochIndex(t time.Time) int64 {
	return int64(t.Sub(e.start) / e.period)
}

func (e epochInfo) getCurrentEpoch() int64 {
	return e.getEpochIndex(time.Now())
}
