package shared

import (
	"container/list"
	"flare-indexer/utils"

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

func NewInputList(inputs []Input) InputList {
	list := InputList{list.New()}
	for _, in := range inputs {
		if !in.ImportInput() {
			list.inputs.PushBack(in)
		}
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
		in := e.Value.(Input)
		if out, ok := outputs.Get(IdIndexKey{in.OutTx(), in.OutIndex()}); ok {
			if out == nil {
				// Genesis tx
				in.UpdateAddr(in.OutTx())
			} else {
				in.UpdateAddr(out.Addr())
			}
			il.inputs.Remove(e)
		} else {
			missingTxIds.Add(in.OutTx())
		}
		e = next
	}
	return missingTxIds
}

func NewOutputMap() OutputMap {
	return make(map[IdIndexKey]Output)
}

func (om OutputMap) Add(k IdIndexKey, o Output) {
	om[k] = o
}

func (om OutputMap) Get(k IdIndexKey) (v Output, ok bool) {
	v, ok = om[k]
	return
}

func NewIdIndexKey(id string, index uint32) IdIndexKey {
	return IdIndexKey{id, index}
}

func NewIdIndexKeyFromOutput(out Output) IdIndexKey {
	return IdIndexKey{out.Tx(), out.Index()}
}
