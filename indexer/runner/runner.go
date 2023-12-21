package runner

import (
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/cronjob"
	"flare-indexer/indexer/pchain"
	"flare-indexer/indexer/xchain"
	"log"
)

func Start(ctx context.IndexerContext) {
	xIndexer := xchain.CreateXChainTxIndexer(ctx)
	pIndexer := pchain.CreatePChainBlockIndexer(ctx)

	mirrorCronjob, err := cronjob.NewMirrorCronjob(ctx)
	if err != nil {
		log.Fatal(err)
	}
	uptimeCronjob := cronjob.NewUptimeCronjob(ctx)
	uptimeVotingCronjob, err := cronjob.NewUptimeVotingCronjob(ctx)
	if err != nil {
		log.Fatal(err)
	}

	go xIndexer.Run()
	go pIndexer.Run()

	go cronjob.RunCronjob(uptimeCronjob)
	go cronjob.RunCronjob(mirrorCronjob)
	go cronjob.RunCronjob(uptimeVotingCronjob)
}
