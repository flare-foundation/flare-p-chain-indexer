package shared

import (
	"bytes"
	"flare-indexer/database"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	avaxutils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

const testHRP = "costwo"

func newTestKey(t *testing.T) (*secp256k1.PrivateKey, ids.ShortID, string) {
	t.Helper()

	key, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := key.PublicKey().Address()
	formatted, err := chain.FormatAddressBytes(addr.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return key, addr, formatted
}

// Sorted, unique owner set as avalanchego requires it.
func newOwners(t *testing.T, threshold uint32, addrs ...ids.ShortID) secp256k1fx.OutputOwners {
	t.Helper()

	owners := secp256k1fx.OutputOwners{Threshold: threshold, Addrs: addrs}
	avaxutils.Sort(owners.Addrs)
	if err := owners.Verify(); err != nil {
		t.Fatalf("owner set rejected by avalanchego: %v", err)
	}
	return owners
}

func ownerSlotOf(t *testing.T, owners secp256k1fx.OutputOwners, addr ids.ShortID) uint32 {
	t.Helper()

	for i, a := range owners.Addrs {
		if a == addr {
			return uint32(i)
		}
	}
	t.Fatalf("address %s is not an owner", addr)
	return 0
}

// Run the funding output and the consuming input through the real indexing pipeline:
// CreateTransferableOutputs -> UpdateWithOutputs -> ToDbInputs.
func indexInput(
	t *testing.T,
	txID string,
	out *secp256k1fx.TransferOutput,
	in *avax.TransferableInput,
) []*database.TxInput {
	t.Helper()

	dbOuts, err := CreateTransferableOutputs(in.TxID.String(), in.OutputIndex, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbOuts) != len(out.Addrs) {
		t.Fatalf("expected %d output rows, got %d", len(out.Addrs), len(dbOuts))
	}

	cache := utils.NewCache[IdIndexKey, Output]()
	for _, dbOut := range dbOuts {
		// Same wrapping the P-chain entity creator does.
		o := &database.PChainTxOutput{TxOutput: *dbOut}
		cache.Add(NewIdIndexKeyFromOutput(o), o)
	}

	ins := InputsFromTxIns(txID, []*avax.TransferableInput{in})
	if missing := NewInputList(ins).UpdateWithOutputs(cache); missing.Cardinality() != 0 {
		t.Fatalf("unresolved inputs: %v", missing)
	}
	return ins[0].ToDbInputs()
}

func transferableInput(fundingTxID ids.ID, amount uint64, sigIndices []uint32) *avax.TransferableInput {
	return &avax.TransferableInput{
		UTXOID: avax.UTXOID{TxID: fundingTxID, OutputIndex: 0},
		Asset:  avax.Asset{ID: ids.GenerateTestID()},
		In: &secp256k1fx.TransferInput{
			Amt:   amount,
			Input: secp256k1fx.Input{SigIndices: sigIndices},
		},
	}
}

func inputAddresses(ins []*database.TxInput) []string {
	addrs := make([]string, len(ins))
	for i, in := range ins {
		addrs[i] = in.Address
	}
	return addrs
}

func TestSingleOwnerInputUnchanged(t *testing.T) {
	chain.AddressHRP = testHRP

	_, addr, formatted := newTestKey(t)
	amount := 100 * units.Avax
	out := &secp256k1fx.TransferOutput{Amt: amount, OutputOwners: newOwners(t, 1, addr)}
	in := transferableInput(ids.GenerateTestID(), amount, []uint32{0})

	dbIns := indexInput(t, ids.GenerateTestID().String(), out, in)
	if got := inputAddresses(dbIns); len(got) != 1 || got[0] != formatted {
		t.Fatalf("expected [%s], got %v", formatted, got)
	}
}

func TestThresholdTwoKeepsBothSigners(t *testing.T) {
	chain.AddressHRP = testHRP

	_, addrA, strA := newTestKey(t)
	_, addrB, strB := newTestKey(t)
	_, addrC, strC := newTestKey(t)

	amount := 100 * units.Avax
	owners := newOwners(t, 2, addrA, addrB, addrC)
	out := &secp256k1fx.TransferOutput{Amt: amount, OutputOwners: owners}

	// A and C sign; B is a listed co-owner that did not.
	sigIndices := []uint32{ownerSlotOf(t, owners, addrA), ownerSlotOf(t, owners, addrC)}
	if sigIndices[0] > sigIndices[1] {
		sigIndices[0], sigIndices[1] = sigIndices[1], sigIndices[0]
	}
	in := transferableInput(ids.GenerateTestID(), amount, sigIndices)

	got := inputAddresses(indexInput(t, ids.GenerateTestID().String(), out, in))
	if len(got) != 2 {
		t.Fatalf("expected 2 signer rows, got %d: %v", len(got), got)
	}
	for _, addr := range got {
		if addr != strA && addr != strC {
			t.Fatalf("unexpected address %s in %v", addr, got)
		}
		if addr == strB {
			t.Fatalf("non-signing co-owner %s was indexed", strB)
		}
	}
}

// Owner slots are ordered by the raw 20 byte address, not by the formatted bech32
// string. The two orderings genuinely differ, because bech32's character set is not in
// ASCII order, so sorting the formatted strings selects the wrong owner.
func TestOwnerOrderIsByAddressBytesNotBech32(t *testing.T) {
	chain.AddressHRP = testHRP

	var first, second ids.ShortID
	var firstStr string
	found := false
	for i := 0; i < 1000 && !found; i++ {
		_, addrA, strA := newTestKey(t)
		_, addrB, strB := newTestKey(t)

		// Want byte order and string order to disagree.
		if bytes.Compare(addrA.Bytes(), addrB.Bytes()) < 0 && strA > strB {
			first, firstStr, second, found = addrA, strA, addrB, true
		} else if bytes.Compare(addrB.Bytes(), addrA.Bytes()) < 0 && strB > strA {
			first, firstStr, second, found = addrB, strB, addrA, true
		}
	}
	if !found {
		t.Fatal("could not find a pair whose byte and bech32 orderings disagree")
	}

	amount := 100 * units.Avax
	owners := newOwners(t, 1, first, second)
	out := &secp256k1fx.TransferOutput{Amt: amount, OutputOwners: owners}

	// Owner slot 0 is `first` by byte order. A string sort would put it last.
	in := transferableInput(ids.GenerateTestID(), amount, []uint32{0})

	got := inputAddresses(indexInput(t, ids.GenerateTestID().String(), out, in))
	if len(got) != 1 || got[0] != firstStr {
		t.Fatalf("expected signer slot 0 to resolve to %s, got %v", firstStr, got)
	}
}

// The two genesis branches of UpdateWithOutputs must behave exactly as before: an empty
// output list falls back to the output transaction id, and the P-chain genesis marker
// (a single nil output) yields no input rows at all.
func TestGenesisInputsUnchanged(t *testing.T) {
	chain.AddressHRP = testHRP

	fundingTxID := ids.GenerateTestID()
	key := NewIdIndexKey(fundingTxID.String(), 0)

	t.Run("empty output list", func(t *testing.T) {
		outs := NewOutputMap()
		outs[key] = []Output{}

		ins := InputsFromTxIns(ids.GenerateTestID().String(),
			[]*avax.TransferableInput{transferableInput(fundingTxID, 1, []uint32{0})})
		if missing := NewInputList(ins).UpdateWithOutputs(outs); missing.Cardinality() != 0 {
			t.Fatalf("unresolved inputs: %v", missing)
		}

		got := inputAddresses(ins[0].ToDbInputs())
		if len(got) != 1 || got[0] != fundingTxID.String() {
			t.Fatalf("expected [%s], got %v", fundingTxID, got)
		}
	})

	t.Run("nil output marker", func(t *testing.T) {
		outs := NewOutputMap()
		outs.Add(key, nil)

		ins := InputsFromTxIns(ids.GenerateTestID().String(),
			[]*avax.TransferableInput{transferableInput(fundingTxID, 1, []uint32{0})})
		if missing := NewInputList(ins).UpdateWithOutputs(outs); missing.Cardinality() != 0 {
			t.Fatalf("unresolved inputs: %v", missing)
		}

		if got := ins[0].ToDbInputs(); len(got) != 0 {
			t.Fatalf("expected no input rows, got %v", inputAddresses(got))
		}
	})
}
