package goCacheMap

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	Version = "v0.0.2"
)

type cacheMap struct {
	mu              sync.RWMutex     // 读写锁
	m               map[string]*Item // Map
	cleanUpInterval time.Duration    // 自动清理过期时间
	stopChan        chan bool        // 停止信号通道
	stopStatus      bool             // 停止状态
	ctx             context.Context
}

type Item struct {
	Key             string          // Key
	Value           interface{}     // Value
	TTL             time.Duration   // 过期时间
	CleanUpCallFunc CleanUpCallFunc // 清理回调函数
	addTime         time.Time       // 添加时间
}

type Option struct {
	CleanUpInterval time.Duration // 自动清理过期时间
	Ctx             context.Context
}

type CleanUpCallFunc func(key string, value interface{}, ttl time.Duration, addTime time.Time) // 清理回调函数类型

type ForEachFunc func(key string, value interface{}, ttl time.Duration, cleanUpCallFunc CleanUpCallFunc, addTime time.Time)

type Error error

var (
	ErrKeyExist    = errors.New("key exist")
	ErrKeyNotExist = errors.New("key not exist")
)

type CacheMap struct {
	*cacheMap
}

func newCacheMap(options ...Option) *cacheMap {
	option := Option{
		CleanUpInterval: 5 * time.Second,
	}
	if options != nil && len(options) > 0 {
		for _, v := range options {
			if v.CleanUpInterval > 1*time.Second {
				option.CleanUpInterval = v.CleanUpInterval
			}
			if v.Ctx != nil {
				option.Ctx = v.Ctx
			}
		}
	}
	if option.Ctx == nil {
		option.Ctx = context.Background()
	}
	return &cacheMap{
		mu:              sync.RWMutex{},
		m:               make(map[string]*Item),
		cleanUpInterval: option.CleanUpInterval,
		stopChan:        make(chan bool),
		stopStatus:      false,
		ctx:             option.Ctx,
	}
}

func (c *cacheMap) autoCleanUp() {
	for {
		select {
		case <-time.After(c.cleanUpInterval):
			c.mu.Lock()
			for k, v := range c.m {
				if v.TTL > 0 && time.Now().Sub(v.addTime) > v.TTL {
					if v.CleanUpCallFunc != nil {
						go v.CleanUpCallFunc(v.Key, v.Value, v.TTL, v.addTime)
					}
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopChan:
			c.stopStatus = true
			return
		case <-c.ctx.Done():
			if c.stopChan != nil {
				close(c.stopChan)
			}
			c.stopStatus = true
			return
		}
	}
}

func NewCacheMap(options ...Option) CacheMap {
	c := newCacheMap(options...)
	go c.autoCleanUp()
	return CacheMap{c}
}

func (c *cacheMap) set(key string, value interface{}, ttl time.Duration, cleanUpCallFunc CleanUpCallFunc) error {
	if ttl <= 0 {
		ttl = -1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exist := c.m[key]
	if exist {
		return ErrKeyExist
	}
	item := &Item{
		Key:             key,
		Value:           value,
		TTL:             ttl,
		CleanUpCallFunc: cleanUpCallFunc,
		addTime:         time.Now(),
	}
	c.m[key] = item
	return nil
}

func (C CacheMap) Set(key string, value interface{}, ttl time.Duration, cleanUpCallFunc CleanUpCallFunc) error {
	return C.set(key, value, ttl, cleanUpCallFunc)
}

func (c *cacheMap) getValue(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exist := c.m[key]
	if !exist {
		return nil, ErrKeyNotExist
	}
	return item.Value, nil
}

func (C CacheMap) GetValue(key string) (interface{}, error) {
	return C.getValue(key)
}

func (c *cacheMap) getTTL(key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exist := c.m[key]
	if !exist {
		return 0, ErrKeyNotExist
	}
	return item.TTL, nil
}

func (C CacheMap) GetTTL(key string) (time.Duration, error) {
	return C.getTTL(key)
}

func (c *cacheMap) getAddTime(key string) (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exist := c.m[key]
	if !exist {
		return time.Time{}, ErrKeyNotExist
	}
	return item.addTime, nil
}

func (C CacheMap) GetAddTime(key string) (time.Time, error) {
	return C.getAddTime(key)
}

func (c *cacheMap) getCleanUpCallFunc(key string) (CleanUpCallFunc, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exist := c.m[key]
	if !exist {
		return nil, ErrKeyNotExist
	}
	return item.CleanUpCallFunc, nil
}

func (C CacheMap) GetCleanUpCallFunc(key string) (CleanUpCallFunc, error) {
	return C.getCleanUpCallFunc(key)
}

func (c *cacheMap) delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exist := c.m[key]
	if !exist {
		return ErrKeyNotExist
	}
	delete(c.m, key)
	return nil
}

func (C CacheMap) Delete(key string) error {
	return C.delete(key)
}

func (c *cacheMap) setValue(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, exist := c.m[key]
	if !exist {
		return ErrKeyNotExist
	}
	item.Value = value
	return nil
}

func (C CacheMap) SetValue(key string, value interface{}) error {
	return C.setValue(key, value)
}

func (c *cacheMap) setTTL(key string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = -1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	item, exist := c.m[key]
	if !exist {
		return ErrKeyNotExist
	}
	item.TTL = ttl
	return nil
}

func (C CacheMap) SetTTL(key string, ttl time.Duration) error {
	return C.setTTL(key, ttl)
}

func (c *cacheMap) setCleanUpCallFunc(key string, cleanUpCallFunc CleanUpCallFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, exist := c.m[key]
	if !exist {
		return ErrKeyNotExist
	}
	item.CleanUpCallFunc = cleanUpCallFunc
	return nil
}

func (C CacheMap) SetCleanUpCallFunc(key string, cleanUpCallFunc CleanUpCallFunc) error {
	return C.setCleanUpCallFunc(key, cleanUpCallFunc)
}

func (c *cacheMap) forEach(f ForEachFunc, togo bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if togo {
		for k, v := range c.m {
			go f(k, v.Value, v.TTL, v.CleanUpCallFunc, v.addTime)
		}
	} else {
		for k, v := range c.m {
			f(k, v.Value, v.TTL, v.CleanUpCallFunc, v.addTime)
		}
	}
}

func (C CacheMap) ForEach(f ForEachFunc, togo bool) {
	C.forEach(f, togo)
}

func (c *cacheMap) clean() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]*Item)
}

func (C CacheMap) Clean() {
	C.clean()
}

func (c *cacheMap) exist(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.m[key]
	return ok
}

func (C CacheMap) Exist(key string) bool {
	return C.exist(key)
}

func (c *cacheMap) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopStatus || c.stopChan == nil {
		return
	}
	c.stopChan <- true
	close(c.stopChan)
	c.stopChan = nil
}

func (C CacheMap) Close() {
	C.close()
}
