package runner

import (
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/pchain"
	"flare-indexer/indexer/xchain"
)

func Start(ctx context.IndexerContext) {
	xIndexer := xchain.CreateXChainTxIndexer(ctx)
	pIndexer := pchain.CreatePChainBlockIndexer(ctx)

	go xIndexer.Run()
	go pIndexer.Run()
}
