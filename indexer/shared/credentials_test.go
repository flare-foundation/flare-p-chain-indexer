package shared

import (
	"flare-indexer/utils/chain"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	mapset "github.com/deckarep/golang-set/v2"
)

// Credential of an input as a wallet builds it: one signature per signer over the sha256 of the
// unsigned transaction bytes, in the order of the input's signature indices
func signedCredential(t *testing.T, unsignedBytes []byte, keys ...*secp256k1.PrivateKey) *secp256k1fx.Credential {
	t.Helper()

	txHash := hashing.ComputeHash256(unsignedBytes)
	cred := &secp256k1fx.Credential{Sigs: make([][secp256k1.SignatureLen]byte, len(keys))}
	for i, key := range keys {
		sig, err := key.SignHash(txHash)
		if err != nil {
			t.Fatal(err)
		}
		copy(cred.Sigs[i][:], sig)
	}
	return cred
}

func inputOf(utxoTxID ids.ID, utxoIndex uint32, amount uint64, sigIndices []uint32) *avax.TransferableInput {
	return &avax.TransferableInput{
		UTXOID: avax.UTXOID{TxID: utxoTxID, OutputIndex: utxoIndex},
		Asset:  avax.Asset{ID: ids.GenerateTestID()},
		In:     &secp256k1fx.TransferInput{Amt: amount, Input: secp256k1fx.Input{SigIndices: sigIndices}},
	}
}

func TestUpdateFromCredentialsRecoversSigners(t *testing.T) {
	chain.AddressHRP = testHRP

	keyA, _, strA := newTestKey(t)
	keyB, _, strB := newTestKey(t)
	unsignedBytes := []byte("unsigned transaction bytes")
	chainGeneratedTxID := ids.GenerateTestID()
	ordinaryTxID := ids.GenerateTestID()

	// Input 0 consumes a chain-generated UTXO with two signers, input 1 an ordinary output
	ins := InputsFromTxIns(ids.GenerateTestID().String(),
		[]*avax.TransferableInput{
			inputOf(chainGeneratedTxID, 1, 7, []uint32{0, 1}),
			inputOf(ordinaryTxID, 0, 5, []uint32{0}),
		},
		NewCredentials([]verify.Verifiable{
			signedCredential(t, unsignedBytes, keyA, keyB),
			signedCredential(t, unsignedBytes, keyA),
		}, unsignedBytes))

	list := NewInputList(ins)
	missing := list.UpdateFromCredentials(mapset.NewSet(NewIdIndexKey(chainGeneratedTxID.String(), 1)))
	if !missing.Equal(mapset.NewSet(ordinaryTxID.String())) {
		t.Fatalf("expected only the ordinary output to remain unresolved, got %v", missing)
	}

	got := inputAddresses(ins[0].ToDbInputs())
	if len(got) != 2 || got[0] != strA || got[1] != strB {
		t.Fatalf("expected signers [%s %s], got %v", strA, strB, got)
	}
	if rows := ins[0].ToDbInputs(); rows[0].Amount != 7 || rows[0].OutTxID != chainGeneratedTxID.String() || rows[0].OutIdx != 1 {
		t.Fatalf("unexpected input row %+v", rows[0])
	}
	if got := ins[1].ToDbInputs(); len(got) != 0 {
		t.Fatalf("ordinary input must not be attributed, got %v", inputAddresses(got))
	}
}

func TestUpdateFromCredentialsOnlyNamedUTXOs(t *testing.T) {
	chain.AddressHRP = testHRP

	keyA, _, _ := newTestKey(t)
	unsignedBytes := []byte("unsigned transaction bytes")
	txID := ids.GenerateTestID()

	// The input consumes output 0 of the tx, only output 1 is chain-generated
	ins := InputsFromTxIns(ids.GenerateTestID().String(),
		[]*avax.TransferableInput{inputOf(txID, 0, 1, []uint32{0})},
		NewCredentials([]verify.Verifiable{signedCredential(t, unsignedBytes, keyA)}, unsignedBytes))

	missing := NewInputList(ins).UpdateFromCredentials(mapset.NewSet(NewIdIndexKey(txID.String(), 1)))
	if !missing.Equal(mapset.NewSet(txID.String())) {
		t.Fatalf("expected the input to remain unresolved, got %v", missing)
	}
	if got := ins[0].ToDbInputs(); len(got) != 0 {
		t.Fatalf("input must not be attributed, got %v", inputAddresses(got))
	}
}

func TestUpdateFromCredentialsWithoutCredentials(t *testing.T) {
	chain.AddressHRP = testHRP

	txID := ids.GenerateTestID()
	ins := InputsFromTxIns(ids.GenerateTestID().String(),
		[]*avax.TransferableInput{inputOf(txID, 1, 1, []uint32{0})}, nil)

	missing := NewInputList(ins).UpdateFromCredentials(mapset.NewSet(NewIdIndexKey(txID.String(), 1)))
	if !missing.Equal(mapset.NewSet(txID.String())) {
		t.Fatalf("expected the input to remain unresolved, got %v", missing)
	}
	if err := ins[0].UpdateAddressesFromCredential(); err == nil {
		t.Fatal("expected an error without credentials")
	}
}
