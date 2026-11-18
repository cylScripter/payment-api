package initials

import (
	"errors"
	"payment-api/internal/service/merchant"
	"payment-api/internal/service/notify"
	"sync"

	"payment-api/internal/config"
	"payment-api/internal/pkg/gormhelper"
	"payment-api/internal/pkg/wechat"
)

var (
	once        sync.Once
	initErr     error
	stopWatch   func()
	stopNotify  func()
	stopAPIKeys func()
	closeMu     sync.Mutex
)

func Init(sources ...config.Source) error {
	once.Do(func() {
		if initErr = InitLogger(); initErr != nil {
			return
		}
		if initErr = config.Init(sources...); initErr != nil {
			return
		}
		if initErr = wechat.Init(); initErr != nil {
			return
		}
		if initErr = InitDatabase(config.Get()); initErr != nil {
			return
		}
		stopAPIKeys, initErr = startAPIKeySync()
		if initErr != nil {
			_ = gormhelper.ClientInstance.Close()
			return
		}
		stopWatch, initErr = config.Watch(applyConfig)
		if initErr != nil {
			stopAPIKeys()
			_ = gormhelper.ClientInstance.Close()
			return
		}
		stopNotify = notify.StartBusinessNotify()
	})

	if initErr != nil {
		return initErr
	}

	merchant.TriggerCache()
	go merchant.TriggerCacheLoop()

	return nil
}

func Close() error {
	closeMu.Lock()
	defer closeMu.Unlock()
	if stopNotify != nil {
		stopNotify()
		stopNotify = nil
	}
	if stopWatch != nil {
		stopWatch()
		stopWatch = nil
	}
	if stopAPIKeys != nil {
		stopAPIKeys()
		stopAPIKeys = nil
	}
	return errors.Join(gormhelper.ClientInstance.Close(), logFile.Close())
}
