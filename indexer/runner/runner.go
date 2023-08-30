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

	votingCronjob, err := cronjob.NewVotingCronjob(ctx)
	if err != nil {
		log.Fatal(err)
	}
	mirrorCronjob, err := cronjob.NewMirrorCronjob(ctx)
	if err != nil {
		log.Fatal(err)
	}

	go xIndexer.Run()
	go pIndexer.Run()

	uptimeCronjob := cronjob.NewUptimeCronjob(ctx)

	go cronjob.RunCronjob(uptimeCronjob)
	go cronjob.RunCronjob(votingCronjob)
	go cronjob.RunCronjob(mirrorCronjob)
}
