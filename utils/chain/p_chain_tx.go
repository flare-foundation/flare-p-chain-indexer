package chain

import (
	"encoding/hex"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/proposervm/block"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/pkg/errors"
)

var (
	ErrInvalidBlockType             = errors.New("invalid block type")
	ErrInvalidTransactionBlock      = errors.New("transaction not found in block")
	ErrInvalidCredentialType        = errors.New("invalid credential type")
	ErrCredentialForAddressNotFound = errors.New("public key not found for address")
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
// a
func PublicKeyFromPChainBlock(txID string, addrBytes [20]byte, addrIndex uint32, blockBytes []byte) (crypto.PublicKey, error) {
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
		if len(blk.Tx.Creds) <= int(addrIndex) {
			return nil, fmt.Errorf("invalid credential index %d", addrIndex)
		}
		txBytes := blk.Tx.Unsigned.Bytes()
		return PublicKeyForAddressAndSignedHash(blk.Tx.Creds[addrIndex], addrBytes, hashing.ComputeHash256(txBytes))
	case *blocks.BanffStandardBlock:
		// In Banff blocks, add delegator and add validator transactions
		// are in standard blocks. We extract public keys from them.
		for _, tx := range blk.Txs() {
			if tx.ID().String() == txID {
				if len(tx.Creds) <= int(addrIndex) {
					return nil, fmt.Errorf("invalid credential index %d", addrIndex)
				}

				// Try with avalanche-style signature
				txHash := hashing.ComputeHash256(tx.Unsigned.Bytes())
				pk, err := PublicKeyForAddressAndSignedHash(tx.Creds[addrIndex], addrBytes, txHash)
				if err == nil {
					return pk, nil
				}

				// Try with eth-style signature
				txHashStr := hex.EncodeToString(txHash)
				txHashEth := TextHash([]byte(txHashStr))
				return PublicKeyForAddressAndSignedHash(tx.Creds[addrIndex], addrBytes, txHashEth)
			}
		}
		return nil, ErrInvalidTransactionBlock
	default:
		return nil, ErrInvalidBlockType
	}
}

// For a given P-chain transaction hash return a public key for
// a signature of a transaction hash that matches the provided address
func PublicKeyForAddressAndSignedHash(cred verify.Verifiable, address [20]byte, signedTxHash []byte) (crypto.PublicKey, error) {
	factory := crypto.FactorySECP256K1R{}
	if secpCred, ok := cred.(*secp256k1fx.Credential); !ok {
		return nil, ErrInvalidCredentialType
	} else {
		sigs := secpCred.Sigs
		for si, sig := range sigs {
			pubKey, err := factory.RecoverHashPublicKey(signedTxHash, sig[:])
			if err != nil {
				return nil, fmt.Errorf("failed to recover public key from cred sig %d: %w", si, err)
			}
			if pubKey.Address() == address {
				return pubKey, nil
			}
		}
		return nil, ErrCredentialForAddressNotFound
	}
}
