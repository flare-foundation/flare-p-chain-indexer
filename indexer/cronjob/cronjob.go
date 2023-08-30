package cronjob

import (
	"flare-indexer/logger"
	"time"
)

type Cronjob interface {
	Name() string
	Enabled() bool
	TimeoutSeconds() int
	Call() error
}

func RunCronjob(c Cronjob) {
	if !c.Enabled() {
		return
	}

	logger.Debug("starting %s cronjob", c.Name())

	ticker := time.NewTicker(time.Duration(c.TimeoutSeconds() * int(time.Second)))
	for range ticker.C {
		err := c.Call()
		if err != nil {
			logger.Error("%s cronjob error %v", c.Name, err)
		}
	}
}
