package chain

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/proposervm/block"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/pkg/errors"
)

var (
	ErrInvalidBlockType        = errors.New("invalid block type")
	ErrInvalidTransactionBlock = errors.New("transaction not found in block")
	ErrInvalidCredentialType   = errors.New("invalid credential type")
)

// If block.Parse fails, try to parse as a "pre-fork" block
func ParsePChainBlock(blockBytes []byte) (blocks.Block, error) {
	blk, err := block.Parse(blockBytes)
	var innerBlk blocks.Block
	if err == nil {
		innerBlk, err = blocks.Parse(blocks.GenesisCodec, blk.Block())
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse inner block")
		}
	} else {
		// try to parse as as a "pre-fork" block
		innerBlk, err = blocks.Parse(blocks.GenesisCodec, blockBytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse block")
		}
	}
	return innerBlk, nil
}

// For a given block (byte array) return a list of public keys for
// signatures of inputs of the transaction with txID in this block
func PublicKeysFromPChainBlock(txID string, blockBytes []byte) ([][]crypto.PublicKey, error) {
	innerBlk, err := ParsePChainBlock(blockBytes)
	if err != nil {
		return nil, err
	}

	switch blk := innerBlk.(type) {
	case *blocks.ApricotProposalBlock:
		// We extract public keys from the add delegator and
		// add validator which are only in proposal blocks
		if blk.Tx.ID().String() != txID {
			return nil, ErrInvalidTransactionBlock
		}
		return PublicKeysFromPChainTx(blk.Tx)
	case *blocks.BanffStandardBlock:
		// In Banff blocks, add delegator and add validator transactions
		// are in standard blocks. We extract public keys from them.
		for _, tx := range blk.Txs() {
			if tx.ID().String() == txID {
				return PublicKeysFromPChainTx(tx)
			}
		}
		return nil, ErrInvalidTransactionBlock
	default:
		return nil, ErrInvalidBlockType
	}
}

// For a given P-chain transaction return a list of public keys for
// signatures of inputs of this transaction
func PublicKeysFromPChainTx(tx *txs.Tx) ([][]crypto.PublicKey, error) {
	creds := tx.Creds
	factory := crypto.FactorySECP256K1R{}
	response := make([][]crypto.PublicKey, len(creds))
	for ci, cred := range creds {
		if secpCred, ok := cred.(*secp256k1fx.Credential); !ok {
			return nil, ErrInvalidCredentialType
		} else {
			sigs := secpCred.Sigs
			response[ci] = make([]crypto.PublicKey, len(sigs))
			for si, sig := range sigs {
				pubKey, err := factory.RecoverPublicKey(tx.Unsigned.Bytes(), sig[:])
				if err != nil {
					return nil, fmt.Errorf("failed to recover public key from cred %d sig %d: %w", ci, si, err)
				}
				response[ci][si] = pubKey
			}
		}
	}
	return response, nil
}
