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
	OnStart() error
}

func RunCronjob(c Cronjob) {
	if !c.Enabled() {
		return
	}
	err := c.OnStart()
	if err != nil {
		logger.Error("%s cronjob on start error %v", c.Name(), err)
		return
	}
	ticker := time.NewTicker(time.Duration(c.TimeoutSeconds() * int(time.Second)))
	for range ticker.C {
		err := c.Call()
		if err != nil {
			logger.Error("%s cronjob call error %v", c.Name, err)
		}
	}
}
