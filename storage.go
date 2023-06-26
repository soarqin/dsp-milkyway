package main

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

type Storage struct {
	user        *redis.Client
	record      *redis.Client
	recordDirty bool
	updateTimer *time.Ticker
	done        chan interface{}
}

type User struct {
	Id       uint64
	LoginKey uint32
}

type Record struct {
	Seed    uint32
	Stars   uint16
	ResMult uint8
	Name    string
	Power   uint64
}

var ctx = context.Background()
var lastRankUrl = ""

func NewStorage() (*Storage, error) {
	user := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	if err := user.Ping(ctx).Err(); err != nil {
		_ = user.Close()
		return nil, err
	}
	record := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	if err := record.Ping(ctx).Err(); err != nil {
		_ = user.Close()
		_ = record.Close()
		return nil, err
	}
	stor := &Storage{user: user, record: record, recordDirty: true}
	stor.UpdateRank()

	stor.updateTimer = time.NewTicker(5 * time.Minute)
	stor.done = make(chan interface{})
	go func() {
		for {
			select {
			case <-stor.updateTimer.C:
				stor.UpdateRank()
			case <-stor.done:
				return
			}
		}
	}()
	return stor, nil
}

func (stor *Storage) Close() {
	stor.updateTimer.Stop()
	stor.done <- struct{}{}
	stor.updateTimer = nil
	close(stor.done)
	_ = stor.user.Close()
	_ = stor.record.Close()
	stor.user = nil
	stor.record = nil
}

func (stor *Storage) AssignLoginKey(userId string, key uint32) {
	stor.user.Set(ctx, "login-"+userId, key, 1800*time.Second)
}

func (stor *Storage) UpdateRank() {
	if !stor.recordDirty {
		return
	}
	rankUrl := time.Now().Format("20060102150405")
	lastRankUrl = rankUrl
	stor.recordDirty = false
}

func (stor *Storage) GetLastRankUrl() string {
	return lastRankUrl
}
