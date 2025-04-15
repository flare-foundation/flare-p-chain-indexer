package staking

import (
	"flare-indexer/database"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/merkle"
	"sort"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
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

func GetMerkleProof(merkleTree merkle.Tree, tx *database.PChainTxData) ([][32]byte, error) {
	hash, err := HashTransaction(tx)
	if err != nil {
		return nil, err
	}

	proof, err := merkleTree.GetProofFromHash(hash)
	if err != nil {
		return nil, errors.Wrap(err, "merkleTree.GetProof")
	}

	proofBytes := make([][32]byte, len(proof))
	for i := range proof {
		proofBytes[i] = [32]byte(proof[i])
	}

	return proofBytes, nil
}

func HashTransaction(tx *database.PChainTxData) (common.Hash, error) {
	if tx.TxID == nil {
		return common.Hash{}, errors.New("tx.TxID is nil")
	}

	encodedBytes, err := encodeTreeItem(tx)
	if err != nil {
		return common.Hash{}, errors.Wrap(err, "encodeTreeItem")
	}

	return crypto.Keccak256Hash(encodedBytes), nil
}

func ToStakeData(
	tx *database.PChainTxData,
) (*mirroring.IPChainStakeMirrorVerifierPChainStake, error) {
	txHash, err := ids.FromString(*tx.TxID)
	if err != nil {
		return nil, errors.Wrap(err, "ids.FromString")
	}

	txType, err := GetTxType(tx.Type)
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

	address, err := chain.ParseAddress(tx.InputAddress)
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

	stakeData, err := ToStakeData(tx)
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

func GetTxType(txType database.PChainTxType) (uint8, error) {
	switch txType {
	case database.PChainAddValidatorTx, database.PChainAddPermissionlessValidatorTx:
		return 0, nil
	case database.PChainAddDelegatorTx, database.PChainAddPermissionlessDelegatorTx:
		return 1, nil
	default:
		return 0, errors.New("invalid tx type")
	}
}

func BuildTree(txs []database.PChainTxData) (merkle.Tree, error) {
	hashes := make([]common.Hash, len(txs))

	for i := range txs {
		hash, err := HashTransaction(&txs[i])
		if err != nil {
			return merkle.Tree{}, errors.Wrap(err, "getTxHash")
		}

		hashes[i] = hash
	}

	return merkle.Build(hashes, false), nil
}

func GetMerkleRoot(votingData []database.PChainTxData) (common.Hash, error) {
	tree, err := BuildTree(votingData)
	if err != nil {
		return [32]byte{}, err
	}
	return tree.Root()
}

// Keep only transactions with unique address and input index 0
func DedupeTxs(txs []database.PChainTxData) []database.PChainTxData {
	txSet := make(map[string]*database.PChainTxData, len(txs))

	for i := range txs {
		tx := &txs[i]
		if tx.TxID == nil || tx.InputIndex != 0 {
			continue
		}

		txSet[*tx.TxID] = tx
	}

	dedupedTxs := make([]database.PChainTxData, 0, len(txSet))
	for _, tx := range txSet {
		dedupedTxs = append(dedupedTxs, *tx)
	}

	// Sort txs lexically by txID. This isn't strictly necessary but provides
	// a consistent ordering for testing.
	sort.Slice(dedupedTxs, func(i, j int) bool {
		return *dedupedTxs[i].TxID < *dedupedTxs[j].TxID
	})

	return dedupedTxs
}
