package xchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils/chain"
	"fmt"

	"github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/wallet/chain/x"
	mapset "github.com/deckarep/golang-set/v2"
	"gorm.io/gorm"
)

type xChainInputUpdater struct {
	shared.BaseInputUpdater

	db     *gorm.DB
	client chain.IndexerClient
}

func newXChainInputUpdater(ctx context.IndexerContext, client chain.IndexerClient) *xChainInputUpdater {
	ioUpdater := xChainInputUpdater{
		db:     ctx.DB(),
		client: client,
	}
	ioUpdater.InitCache()
	return &ioUpdater
}

func (iu *xChainInputUpdater) UpdateInputs(inputs shared.InputList) (mapset.Set[string], error) {
	missingTxIds := iu.UpdateInputsFromCache(inputs)
	missingTxIds, err := iu.updateFromDB(inputs, missingTxIds)
	if err != nil {
		return nil, err
	}
	return iu.updateFromChain(inputs, missingTxIds)
}

func (iu *xChainInputUpdater) updateFromDB(
	inputs shared.InputList,
	missingTxIds mapset.Set[string],
) (mapset.Set[string], error) {
	outs, err := database.FetchXChainTxOutputs(iu.db, missingTxIds.ToSlice())
	if err != nil {
		return nil, err
	}
	baseOuts := shared.NewOutputMap()
	for _, out := range outs {
		baseOuts.Add(shared.NewIdIndexKey(out.TxID, out.Index()), &out.TxOutput)
	}
	return inputs.UpdateWithOutputs(baseOuts), nil
}

func (iu *xChainInputUpdater) updateFromChain(
	inputs shared.InputList,
	missingTxIds mapset.Set[string],
) (mapset.Set[string], error) {
	fetchedOuts := shared.NewOutputMap()
	for txId := range missingTxIds.Iterator().C {
		container, err := chain.FetchContainerFromIndexer(iu.client, txId)
		if err != nil {
			return nil, err
		}
		if container == nil {
			continue
		}

		tx, err := x.Parser.ParseGenesisTx(container.Bytes)
		if err != nil {
			return nil, err
		}

		var outs []shared.Output
		switch unsignedTx := tx.Unsigned.(type) {
		case *txs.BaseTx:
			outs, err = shared.OutputsFromTxOuts(txId, unsignedTx.Outs, 0, database.DefaultOutput, XChainInputOutputCreator /* TODO could be identity, it is not persisted */)
		case *txs.ImportTx:
			outs, err = shared.OutputsFromTxOuts(txId, unsignedTx.BaseTx.Outs, 0, database.DefaultOutput, XChainInputOutputCreator /* TODO could be identity it is not persisted */)
		default:
			return nil, fmt.Errorf("transaction with id %s has unsupported type %T", container.ID.String(), unsignedTx)
		}
		if err != nil {
			return nil, err
		}
		for _, out := range outs {
			fetchedOuts.Add(shared.NewIdIndexKey(out.Tx(), out.Index()), out)
		}
	}
	return inputs.UpdateWithOutputs(fetchedOuts), nil
}
