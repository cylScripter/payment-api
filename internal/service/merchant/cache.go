package merchant

import (
	"fmt"
	"payment-api/internal/models"
	"payment-api/internal/repo"
	"sync"
	"time"
)

type Cache struct {
	merchant sync.Map
	channel  sync.Map
}

var instance = &Cache{}
var cacheMu sync.RWMutex

func TriggerCache() {
	next := &Cache{}
	mysql := repo.NewMerchantMysql(models.PayDb())
	merchants := mysql.Merchants()
	for _, merchant := range merchants {
		next.merchant.Store(merchant.Account, merchant)
	}
	channels := mysql.MerchantChannels()
	for _, channel := range channels {
		keys := fmt.Sprintf("%d:%s", channel.MerchantID, channel.ChannelCode)
		next.channel.Store(keys, channel)
	}
	cacheMu.Lock()
	instance = next
	cacheMu.Unlock()
}

func TriggerCacheLoop() {
	ticker := time.NewTicker(3 * 60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			TriggerCache()
		}
	}
}

func GetMerchant(account string) models.PayMerchant {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	var merchant models.PayMerchant
	if v, ok := instance.merchant.Load(account); ok {
		m, ok2 := v.(*models.PayMerchant)
		if ok2 {
			merchant = *m
		}
	}
	return merchant
}

func GetChannel(id int, channelCode string) models.PayMerchantChannel {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	var channel models.PayMerchantChannel
	if v, ok := instance.channel.Load(fmt.Sprintf("%d:%s", id, channelCode)); ok {
		c, ok2 := v.(*models.PayMerchantChannel)
		if ok2 {
			channel = *c
		}
	}
	return channel
}
