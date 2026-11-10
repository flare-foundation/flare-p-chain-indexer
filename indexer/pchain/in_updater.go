package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils/chain"

	"github.com/ava-labs/avalanchego/vms/platformvm/platform"
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
	// UTXOs the chain minted outside of the outputs of the fetched transactions, see below
	chainGeneratedUTXOs := mapset.NewSet[shared.IdIndexKey]()
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
		case *platform.AddValidatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *platform.AddPermissionlessValidatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *platform.AddDelegatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *platform.AddPermissionlessDelegatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *platform.AddAutoRenewedValidatorTx:
			outs, err = iu.getAddStakerTxAndRewardTxOutputs(txId, unsignedTx)
		case *platform.RewardAutoRenewedValidatorTx:
			// Reward UTXOs of a RewardAutoRenewedValidatorTx are stored under
			// the reward tx's own id (it has no inputs or outputs of its own)
			outs, err = getRewardOutputs(iu.client, txId)
		case *platform.DisableL1ValidatorTx:
			// Disabling an L1 validator, or removing it with a weight of 0, returns its remaining
			// balance in a UTXO the chain mints at output index len(Outs) of the tx (ACP-77). The UTXO
			// is not among the outputs of the tx and its owner, the remaining balance owner of the L1
			// validator, is kept in the validator state only, which the node drops once the validator
			// is removed. The indexer does not record the UTXO either; an input consuming it is
			// attributed to the signers of its credential instead. The UTXO is minted only if the
			// validator still had a balance, but an input consuming it is proof enough that it was.
			outs, err = shared.OutputsFromTxOuts(txId, unsignedTx.Outs, 0, PChainDefaultInputOutputCreator)
			chainGeneratedUTXOs.Add(shared.NewIdIndexKey(txId, uint32(len(unsignedTx.Outs))))
		case *platform.SetL1ValidatorWeightTx:
			// See DisableL1ValidatorTx
			outs, err = shared.OutputsFromTxOuts(txId, unsignedTx.Outs, 0, PChainDefaultInputOutputCreator)
			chainGeneratedUTXOs.Add(shared.NewIdIndexKey(txId, uint32(len(unsignedTx.Outs))))
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
	notUpdated := inputs.UpdateWithOutputs(fetchedOuts)
	if chainGeneratedUTXOs.Cardinality() > 0 {
		notUpdated = inputs.UpdateFromCredentials(chainGeneratedUTXOs)
	}
	return notUpdated, nil
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
