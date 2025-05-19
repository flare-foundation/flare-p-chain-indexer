package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils/chain"

	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	mapset "github.com/deckarep/golang-set/v2"
	"gorm.io/gorm"
)

type pChainInputUpdater struct {
	shared.BaseInputUpdater

	db     *gorm.DB
	client chain.RPCClient
}

func newPChainInputUpdater(ctx context.IndexerContext, client chain.RPCClient) *pChainInputUpdater {
	ioUpdater := pChainInputUpdater{
		db:     ctx.DB(),
		client: client,
	}
	ioUpdater.InitCache()
	return &ioUpdater
}

func (iu *pChainInputUpdater) UpdateInputs(inputs shared.InputList) (mapset.Set[string], error) {
	missingTxIds := iu.UpdateInputsFromCache(inputs)
	missingTxIds, err := iu.updateFromDB(inputs, missingTxIds)
	if err != nil {
		return nil, err
	}
	return iu.updateFromChain(inputs, missingTxIds)
}

// notUpdated is a map from *output* id to inputs referring this output
func (iu *pChainInputUpdater) updateFromDB(
	inputs shared.InputList,
	missingTxIds mapset.Set[string],
) (mapset.Set[string], error) {
	outs, err := database.FetchPChainTxOutputs(iu.db, missingTxIds.ToSlice())
	if err != nil {
		return nil, err
	}
	baseOuts := shared.NewOutputMap()
	for i, out := range outs {
		baseOuts.Add(shared.NewIdIndexKey(out.TxID, out.Index()), &outs[i].TxOutput)
	}
	return inputs.UpdateWithOutputs(baseOuts), nil
}

// notUpdated is a map from *output* id to inputs referring this output
func (iu *pChainInputUpdater) updateFromChain(
	inputs shared.InputList,
	missingTxIds mapset.Set[string],
) (mapset.Set[string], error) {
	fetchedOuts := shared.NewOutputMap()
	for txId := range missingTxIds.Iterator().C {
		tx, err := CallPChainGetTxApi(iu.client, txId)
		if err != nil {
			return nil, err
		}
		if tx == nil {
			// Genesis tx
			fetchedOuts.Add(shared.NewIdIndexKey(txId, 0), nil)
			continue
		}

		var outs []shared.Output
		switch unsignedTx := tx.Unsigned.(type) {
		case *txs.AddValidatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *txs.AddDelegatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		default:
			txOuts := tx.Unsigned.Outputs()
			outs, err = shared.OutputsFromTxOuts(txId, txOuts, 0, PChainDefaultInputOutputCreator)
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

func (iu *pChainInputUpdater) getAddStakerTxAndRewardTxOutputs(txId string, tx StakerTx) ([]shared.Output, error) {
	outs, err := getAddStakerTxOutputs(txId, tx)
	if err != nil {
		return nil, err
	}
	rewardOuts, err := getRewardOutputs(iu.client, txId)
	if err != nil {
		return nil, err
	}
	return append(outs, rewardOuts...), nil
}
