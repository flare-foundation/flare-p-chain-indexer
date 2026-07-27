package shared

import (
	"bytes"
	"container/list"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
)

type InputUpdater interface {
	// Update inputs with addresses. Updater can get outputs from cache, db, chain (indexer, api), ...
	// Updated inputs should be removed from the list, missing output tx ids are returned
	UpdateInputs(inputs InputList) (mapset.Set[string], error)

	// Put outputs of a transaction to cache -- to avoid updating from chain or database
	CacheOutputs(outs []Output)
	PurgeCache()
}

type BaseInputUpdater struct {
	cache utils.Cache[IdIndexKey, Output]
}

func (iu *BaseInputUpdater) InitCache() {
	iu.cache = utils.NewCache[IdIndexKey, Output]()
}

func (iu *BaseInputUpdater) CacheOutputs(outs []Output) {
	for _, out := range outs {
		iu.cache.Add(IdIndexKey{out.Tx(), out.Index()}, out)
	}
}

func (iu *BaseInputUpdater) PurgeCache() {
	iu.cache.RemoveAccessed()
}

// Update inputs with addresses from outputs in cache, return missing output tx ids
func (iu *BaseInputUpdater) UpdateInputsFromCache(notUpdated InputList) mapset.Set[string] {
	return notUpdated.UpdateWithOutputs(iu.cache)
}

func NewInputList(inputs []UpdatableInput) InputList {
	list := InputList{list.New()}
	for _, in := range inputs {
		list.inputs.PushBack(in)
	}
	return list
}

// Update input address from outputs
//   - updated inputs will be removed from the list
//   - return missing output tx ids
func (il InputList) UpdateWithOutputs(outputs utils.CacheBase[IdIndexKey, Output]) mapset.Set[string] {
	missingTxIds := mapset.NewSet[string]()
	for e := il.inputs.Front(); e != nil; {
		next := e.Next()
		in := e.Value.(UpdatableInput)
		if outs, ok := outputs.Get(IdIndexKey{in.OutTx(), in.OutIndex()}); ok {
			if len(outs) == 0 {
				// Genesis tx
				in.UpdateAddresses([]string{in.OutTx()})
			} else {
				addresses := make([]string, 0, len(outs))
				for _, out := range outs {
					if out != nil {
						addresses = append(addresses, out.Addr())
					}
				}
				in.UpdateAddresses(addresses)
			}
			il.inputs.Remove(e)
		} else {
			missingTxIds.Add(in.OutTx())
		}
		e = next
	}
	return missingTxIds
}

// Keep only the owners of a consumed output that authorized the spend, i.e. those the
// input's signature indices point at. Spending a multi-owner output does not require a
// signature from every listed owner, so without this an unrelated co-owner could be
// recorded as the spender and end up as the owner of a mirrored stake.
func selectSigners(addresses []string, sigIndices []uint32, outTxID string, outIdx uint32) []string {
	// Nothing to disambiguate: a single owner, or a genesis input whose address is the
	// output transaction id rather than a bech32 address.
	if len(addresses) <= 1 {
		return addresses
	}

	if len(sigIndices) == 0 {
		logger.Warn("no signature indices for output %s:%d with %d owners, keeping all of them",
			outTxID, outIdx, len(addresses))
		return addresses
	}

	type owner struct {
		raw       [20]byte
		formatted string
	}
	owners := make([]owner, len(addresses))
	for i, address := range addresses {
		raw, err := chain.ParseAddress(address)
		if err != nil {
			logger.Warn("unable to parse address %s of output %s:%d, keeping all owners: %s",
				address, outTxID, outIdx, err)
			return addresses
		}
		owners[i] = owner{raw: raw, formatted: address}
	}
	// Signature indices are positions in the output's owner array, which avalanchego
	// requires to be sorted by the raw 20 byte address value. The owners reach us in
	// whatever order the consumed output was resolved in, which need not match -- rows
	// read back from the database are unordered -- so the on-chain order has to be
	// restored before indexing into it. Sorting the formatted bech32 addresses is not
	// equivalent: its character set is not in ASCII order, so it selects a different owner.
	sort.Slice(owners, func(i, j int) bool {
		return bytes.Compare(owners[i].raw[:], owners[j].raw[:]) < 0
	})

	signers := make([]string, len(sigIndices))
	for i, sigIndex := range sigIndices {
		if sigIndex >= uint32(len(owners)) {
			logger.Warn("signature index %d out of range for output %s:%d with %d owners, keeping all of them",
				sigIndex, outTxID, outIdx, len(owners))
			return addresses
		}
		signers[i] = owners[sigIndex].formatted
	}
	return signers
}

func NewOutputMap() OutputMap {
	return make(map[IdIndexKey][]Output)
}

func (om OutputMap) Add(k IdIndexKey, o Output) {
	om[k] = append(om[k], o)
}

func (om OutputMap) Get(k IdIndexKey) (v []Output, ok bool) {
	v, ok = om[k]
	return
}

func NewIdIndexKey(id string, index uint32) IdIndexKey {
	return IdIndexKey{id, index}
}

func NewIdIndexKeyFromOutput(out Output) IdIndexKey {
	return IdIndexKey{out.Tx(), out.Index()}
}
