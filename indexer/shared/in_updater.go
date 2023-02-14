package shared

import (
	"container/list"

	mapset "github.com/deckarep/golang-set/v2"
)

type InputList struct {
	inputs *list.List
}

type InputUpdater interface {
	// Update inputs with addresses. Updater can get outputs from cache, db, chain (indexer, api), ...
	// Updated inputs should be removed from the list, missing output tx ids are returned
	UpdateInputs(inputs InputList) (mapset.Set[string], error)

	// Put outputs of a transaction to cache -- to avoid updating from chain or database
	CacheOutputs(outs []Output)
}

type IdIndexKey struct {
	ID    string
	Index uint32
}

type BaseInputUpdater struct {
	cache map[IdIndexKey]Output
}

func (iu *BaseInputUpdater) InitCache() {
	iu.cache = make(map[IdIndexKey]Output)
}

func (iu *BaseInputUpdater) CacheOutputs(outs []Output) {
	for _, out := range outs {
		iu.cache[IdIndexKey{out.Tx(), out.Index()}] = out
	}
}

// Update inputs with addresses from outputs in cache, return missing output tx ids
func (iu *BaseInputUpdater) UpdateInputsFromCache(notUpdated InputList) mapset.Set[string] {
	return notUpdated.UpdateWithOutputs(iu.cache)
}

func NewInputList(inputs []Input) InputList {
	list := InputList{list.New()}
	for _, in := range inputs {
		list.inputs.PushBack(in)
	}
	return list
}

// Update input address from outputs
//  - updated inputs will be removed from the list
//  - return missing output tx ids
func (il InputList) UpdateWithOutputs(outputs map[IdIndexKey]Output) mapset.Set[string] {
	missingTxIds := mapset.NewSet[string]()
	for e := il.inputs.Front(); e != nil; {
		next := e.Next()
		in := e.Value.(Input)
		if out, ok := outputs[IdIndexKey{in.OutTx(), in.OutIndex()}]; ok {
			in.UpdateAddr(out.Addr())
			il.inputs.Remove(e)
		} else {
			missingTxIds.Add(in.OutTx())
		}
		e = next
	}
	return missingTxIds
}

func NewIdIndexKey(id string, index uint32) IdIndexKey {
	return IdIndexKey{id, index}
}

func NewIdIndexKeyFromOutput(out Output) IdIndexKey {
	return IdIndexKey{out.Tx(), out.Index()}
}
