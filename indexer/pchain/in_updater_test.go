package pchain

import (
	"encoding/hex"
	"encoding/json"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils/chain"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/platform"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// Transactions from Flare mainnet, P-chain blocks 2157431-2157439 (November 2025):
//   - a DisableL1ValidatorTx, which returned the remaining balance of the L1 validator in a UTXO
//     at output index 1, outside of its single declared output;
//   - a CreateSubnetTx consuming that UTXO (output 1), whose spend wedged the indexer;
//   - a CreateSubnetTx consuming the declared output 0 of the disable tx.
//
// The remaining balance owner and the owner of the declared output are the same address.
const (
	disableL1ValidatorTxID  = "9WF7bB3sHQpuHnh6zbGZyKz31J4dZeBBhXf87QXqc1UnvwXGu"
	disableL1ValidatorTxHex = "0000000000270000000e000000000000000000000000000000000000000000000000000000000000000000000001b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af8537824000000070000000039fd12e30000000000000000000000010000000167506324b9244d1c23e1cabe63aaa11ba11f187e00000001fcd2f00d4146b391f4a54a2b7ff73bc80ff8973641b201a595926983960c296500000000b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af853782400000005000000003a597b630000000100000000000000004a16afa9a181706737d71623af5afdae903239fe78e53258fe7ab35255329e540000000a0000000100000000000000020000000900000001ee3a2f45686008a56bba0c7f545a34d606ba52240d9b7a27cf4e14c6019ec621482a3fb313887c0e70044e47617a030c0f7ce1d2b6e7a9cc23a8068a034804ea000000000900000001ee3a2f45686008a56bba0c7f545a34d606ba52240d9b7a27cf4e14c6019ec621482a3fb313887c0e70044e47617a030c0f7ce1d2b6e7a9cc23a8068a034804ea00"

	spendRemainingBalanceTxID  = "2ec82swBMLSjMrcgZK1pBwcwy3T1u3AELBB1pYA93m5kfENFsS"
	spendRemainingBalanceTxHex = "0000000000100000000e000000000000000000000000000000000000000000000000000000000000000000000001b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af8537824000000070000000005ce43e40000000000000000000000010000000167506324b9244d1c23e1cabe63aaa11ba11f187e00000001134f501d21fab6e94bedad38f346330569e9bd9aa578157ed4d423f6433831c600000001b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af8537824000000050000000005f579000000000100000000000000000000000b0000000000000000000000010000000167506324b9244d1c23e1cabe63aaa11ba11f187e000000010000000900000001fef56c1c8bbb9dfc9585f54939464859758fef967e6c4dd47db2df2bc23b282c704a790dc141941c6b7946914cc2eee89d421b22c459bba4a1fffb6a9ba0e41a00"

	spendDeclaredOutputTxID  = "mzH47Fv1gekzDNV6tjbvyAKfN8f49L6mBXquFeVkDwcBHJ5fe"
	spendDeclaredOutputTxHex = "0000000000100000000e000000000000000000000000000000000000000000000000000000000000000000000001b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af8537824000000070000000039d5b5a10000000000000000000000010000000167506324b9244d1c23e1cabe63aaa11ba11f187e00000001134f501d21fab6e94bedad38f346330569e9bd9aa578157ed4d423f6433831c600000000b3462fc39568bf99fd346d3cdcfe1fb900f14cdcf276e3c0d95f814af8537824000000050000000039fd12e30000000100000000000000000000000b0000000000000000000000010000000167506324b9244d1c23e1cabe63aaa11ba11f187e0000000100000009000000011628c84a4db109f0fc4c7e1b5efaf2d3130e8c2f13018b5a690e41bd320fad6d5a5307b40da5a211e3aa4d997dd3646a5c7634d604930e2c221a820435d7cd8c00"

	l1ValidatorOwner = "flare1vagxxf9ey3x3cglpe2lx824prws37xr79uarxl"
)

// RPC client serving the given transactions from memory
type fakeRPCClient struct {
	txs map[ids.ID]*platform.Tx
}

func (c *fakeRPCClient) GetTx(id ids.ID) (*api.GetTxReply, error) {
	tx, ok := c.txs[id]
	if !ok {
		return nil, fmt.Errorf("couldn't get tx: not found")
	}
	txStr, err := formatting.Encode(formatting.Hex, tx.Bytes())
	if err != nil {
		return nil, err
	}
	reply := &api.GetTxReply{Encoding: formatting.Hex}
	reply.Tx, err = json.Marshal(txStr)
	return reply, err
}

func (c *fakeRPCClient) GetRewardUTXOs(id ids.ID) (*chain.GetRewardUTXOsReply, error) {
	return &chain.GetRewardUTXOsReply{Encoding: formatting.Hex}, nil
}

func setAddressHRP(t *testing.T, hrp string) {
	t.Helper()

	previous := chain.AddressHRP
	chain.AddressHRP = hrp
	t.Cleanup(func() { chain.AddressHRP = previous })
}

func parseTx(t *testing.T, txHex string, expectedID string) *platform.Tx {
	t.Helper()

	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := platform.ParseTx(platform.Codec, txBytes)
	if err != nil {
		t.Fatal(err)
	}
	if tx.ID().String() != expectedID {
		t.Fatalf("expected tx %s, got %s", expectedID, tx.ID())
	}
	return tx
}

// Inputs of a signed CreateSubnetTx as the batch indexer creates them
func createSubnetTxInputs(t *testing.T, tx *platform.Tx) []shared.UpdatableInput {
	t.Helper()

	unsignedTx, ok := tx.Unsigned.(*platform.CreateSubnetTx)
	if !ok {
		t.Fatalf("expected a CreateSubnetTx, got %T", tx.Unsigned)
	}
	return shared.InputsFromTxIns(tx.ID().String(), unsignedTx.Ins, shared.NewCredentials(tx.Creds, tx.Unsigned.Bytes()))
}

func newTestInputUpdater(txs ...*platform.Tx) *pChainInputUpdater {
	client := &fakeRPCClient{txs: make(map[ids.ID]*platform.Tx)}
	for _, tx := range txs {
		client.txs[tx.ID()] = tx
	}
	updater := &pChainInputUpdater{client: client}
	updater.InitCache()
	return updater
}

// The spend of the remaining balance of a disabled L1 validator is attributed to the signer of the
// spending input, the spend of a declared output of the disable tx to the owner of the output
func TestDisableL1ValidatorTxSpendsResolvedFromChain(t *testing.T) {
	setAddressHRP(t, "flare")

	disableTx := parseTx(t, disableL1ValidatorTxHex, disableL1ValidatorTxID)
	remainingBalanceSpend := parseTx(t, spendRemainingBalanceTxHex, spendRemainingBalanceTxID)
	declaredOutputSpend := parseTx(t, spendDeclaredOutputTxHex, spendDeclaredOutputTxID)

	updater := newTestInputUpdater(disableTx)
	ins := append(createSubnetTxInputs(t, remainingBalanceSpend), createSubnetTxInputs(t, declaredOutputSpend)...)
	list := shared.NewInputList(ins)

	// Nothing is cached, both inputs refer to the disable tx
	missing := updater.UpdateInputsFromCache(list)
	if missing.Cardinality() != 1 || !missing.Contains(disableL1ValidatorTxID) {
		t.Fatalf("expected the disable tx to be missing, got %v", missing)
	}

	notUpdated, err := updater.updateFromChain(list, missing)
	if err != nil {
		t.Fatal(err)
	}
	if notUpdated.Cardinality() != 0 {
		t.Fatalf("unresolved inputs: %v", notUpdated)
	}

	rows := ins[0].ToDbInputs()
	if len(rows) != 1 {
		t.Fatalf("expected one input row for the remaining balance spend, got %d", len(rows))
	}
	if rows[0].Address != l1ValidatorOwner || rows[0].OutTxID != disableL1ValidatorTxID || rows[0].OutIdx != 1 || rows[0].Amount != 99973376 {
		t.Fatalf("unexpected remaining balance input row %+v", *rows[0])
	}

	rows = ins[1].ToDbInputs()
	if len(rows) != 1 {
		t.Fatalf("expected one input row for the declared output spend, got %d", len(rows))
	}
	if rows[0].Address != l1ValidatorOwner || rows[0].OutTxID != disableL1ValidatorTxID || rows[0].OutIdx != 0 || rows[0].Amount != 972886755 {
		t.Fatalf("unexpected declared output input row %+v", *rows[0])
	}
}

// Only the output index following the declared outputs is chain-generated; a reference to any
// other unknown output of the disable tx must still be reported as unresolved
func TestDisableL1ValidatorTxUnknownOutputStaysUnresolved(t *testing.T) {
	setAddressHRP(t, "flare")

	disableTx := parseTx(t, disableL1ValidatorTxHex, disableL1ValidatorTxID)
	spendTx := parseTx(t, spendRemainingBalanceTxHex, spendRemainingBalanceTxID)

	in := &avax.TransferableInput{
		UTXOID: avax.UTXOID{TxID: disableTx.ID(), OutputIndex: 2},
		Asset:  avax.Asset{ID: ids.GenerateTestID()},
		In:     &secp256k1fx.TransferInput{Amt: 1, Input: secp256k1fx.Input{SigIndices: []uint32{0}}},
	}
	ins := shared.InputsFromTxIns(spendTx.ID().String(), []*avax.TransferableInput{in},
		shared.NewCredentials(spendTx.Creds, spendTx.Unsigned.Bytes()))
	list := shared.NewInputList(ins)

	updater := newTestInputUpdater(disableTx)
	notUpdated, err := updater.updateFromChain(list, updater.UpdateInputsFromCache(list))
	if err != nil {
		t.Fatal(err)
	}
	if notUpdated.Cardinality() != 1 || !notUpdated.Contains(disableL1ValidatorTxID) {
		t.Fatalf("expected the input to remain unresolved, got %v", notUpdated)
	}
	if rows := ins[0].ToDbInputs(); len(rows) != 0 {
		t.Fatalf("input must not be attributed, got %+v", rows)
	}
}
