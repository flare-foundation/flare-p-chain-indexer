package pchain

import (
	"flare-indexer/config"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
)

const (
	StateName          string = "p_chain_block"
	ChainTimeStateName string = "p_chain_time"
)

type pChainBlockIndexer struct {
	shared.ChainIndexerBase
}

func CreatePChainBlockIndexer(ctx context.IndexerContext) *pChainBlockIndexer {
	config := ctx.Config().PChainIndexer
	client := newIndexerClient(&ctx.Config().Chain)
	rpcClient := newJsonRpcClient(&ctx.Config().Chain)

	idxr := pChainBlockIndexer{}
	idxr.StateName = StateName
	idxr.IndexerName = "P-chain Blocks"
	idxr.Client = client
	idxr.DB = ctx.DB()
	idxr.Config = config
	idxr.InitMetrics(StateName)

	idxr.BatchIndexer = NewPChainBatchIndexer(ctx, client, rpcClient, nil)

	return &idxr
}

func (xi *pChainBlockIndexer) Run() {
	xi.ChainIndexerBase.Run()
}

func newIndexerClient(cfg *config.ChainConfig) chain.IndexerClient {
	return chain.NewAvalancheIndexerClient(utils.JoinPaths(cfg.NodeURL, "ext/index/P/block"),
		chain.ClientOptions(cfg.ApiKey)...)
}

func newJsonRpcClient(cfg *config.ChainConfig) chain.RPCClient {
	return chain.NewAvalancheRPCClient(utils.JoinPaths(cfg.NodeURL, "ext/bc/P"+chain.RPCClientOptions(cfg.ApiKey)))
}
