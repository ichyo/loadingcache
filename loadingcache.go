package loadingcache

import (
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/bluele/gcache"
	"github.com/ichyo/loadingcache/singleflight"
)

// An AtomicInt is an int64 to be accessed atomically.
type AtomicInt int64

// Add atomically adds n to i.
func (i *AtomicInt) Add(n int64) {
	atomic.AddInt64((*int64)(i), n)
}

// Get atomically gets the value of i.
func (i *AtomicInt) Get() int64 {
	return atomic.LoadInt64((*int64)(i))
}

func (i *AtomicInt) String() string {
	return strconv.FormatInt(i.Get(), 10)
}

type LoaderFunc[K comparable, V any] func(K) (V, error)

type Config[K comparable, V any] struct {
	// Required: the loading fuction to fill in a value
	Loader LoaderFunc[K, V]
	// Required: the maximum number of items to store in cache.
	MaxItems int

	// expire time from when the load fuction execution started
	// zero value means values won't be expired unless they are manually cleared.
	ExpireAfterLoadStart time.Duration
}

type LoadingCache[K comparable, V any] struct {
	cache                gcache.Cache
	group                singleflight.Group[K, *V]
	loader               LoaderFunc[K, V]
	expireAfterLoadStart time.Duration

	Stats Stats
}

type Stats struct {
	Gets         AtomicInt
	CacheHits    AtomicInt
	Loads        AtomicInt
	LoadsDeduped AtomicInt
}

func NewCache[K comparable, V any](config *Config[K, V]) (*LoadingCache[K, V], error) {
	if config.MaxItems <= 0 {
		return nil, errors.New("MaxItems must be positive")
	}
	if config.Loader == nil {
		return nil, errors.New("Loader must be set")
	}
	if config.ExpireAfterLoadStart < 0 {
		return nil, errors.New("ExpireAfterLoadStart must not be nagative")
	}
	cache := gcache.New(config.MaxItems).ARC().Build()
	return &LoadingCache[K, V]{cache: cache, group: singleflight.Group[K, *V]{}, loader: config.Loader, expireAfterLoadStart: config.ExpireAfterLoadStart}, nil
}

func (c *LoadingCache[K, V]) Get(key K) (*V, error) {
	c.Stats.Gets.Add(1)

	cacheValue, cacheHit := c.getFromInternalCache(key)
	if cacheHit {
		c.Stats.CacheHits.Add(1)
		return cacheValue, nil
	}

	c.Stats.Loads.Add(1)
	value, err, _ := c.group.Do(key, c.expireAfterLoadStart, func() (*V, error) {
		// Check the cache again.
		// See https://github.com/golang/groupcache/blob/master/groupcache.go#L240 for the reason.
		cacheValue, cacheHit := c.getFromInternalCache(key)
		if cacheHit {
			c.Stats.CacheHits.Add(1)
			return cacheValue, nil
		}

		c.Stats.LoadsDeduped.Add(1)
		start := time.Now()

		value, err := c.loader(key)

		if err != nil {
			// TODO: how to handle when loader returns an error? (retryable?)
			// TODO: should this error be wrapped?
			return nil, err
		}

		if c.expireAfterLoadStart <= 0 {
			c.cache.Set(key, &value)
		} else {
			expireTime := start.Add(c.expireAfterLoadStart)
			ttl := expireTime.Sub(time.Now())
			if ttl > 0 {
				c.cache.SetWithExpire(key, &value, ttl)
			}
		}

		return &value, nil
	})

	return value, err
}

func (c *LoadingCache[K, V]) getFromInternalCache(key K) (*V, bool) {
	value, err := c.cache.Get(key)
	if err == nil {
		return value.(*V), true
	} else {
		return nil, false
	}
}
