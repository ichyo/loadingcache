package loadingcache

import (
	"errors"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/ichyo/loadingcache/singleflight"
)

type LoaderFunc[K comparable, V any] func(K) (V, error)

type Config[K comparable, V any] struct {
	// Required: the loading fuction to fill in a value
	Loader LoaderFunc[K, V]
	// Required: the maximum number of items to store in cache.
	MaxItems int64

	// expire time from when the load fuction execution started
	// zero value means values won't be expired unless they are manually cleared.
	ExpireAfterLoadStart time.Duration
}

type LoadingCache[K comparable, V any] struct {
	cache                *ristretto.Cache
	group                singleflight.Group[K, *V]
	loader               LoaderFunc[K, V]
	expireAfterLoadStart time.Duration
}

func NewCache[K comparable, V any](config Config[K, V]) (*LoadingCache[K, V], error) {
	if config.MaxItems <= 0 {
		return nil, errors.New("MaxItems must be positive")
	}
	if config.Loader == nil {
		return nil, errors.New("Loader must be set")
	}
	if config.ExpireAfterLoadStart < 0 {
		return nil, errors.New("ExpireAfterLoadStart must not be nagative")
	}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: config.MaxItems * 10,
		MaxCost:     config.MaxItems,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &LoadingCache[K, V]{cache: cache, group: singleflight.Group[K, *V]{}, loader: config.Loader, expireAfterLoadStart: config.ExpireAfterLoadStart}, nil
}

func (c *LoadingCache[K, V]) Get(key K) (*V, error) {
	cacheValue, cacheHit := c.getFromInternalCache(key)
	if cacheHit {
		return cacheValue, nil
	}

	value, err, _ := c.group.Do(key, c.expireAfterLoadStart, func() (*V, error) {
		// Check the cache again.
		// See https://github.com/golang/groupcache/blob/master/groupcache.go#L240 for the reason.
		cacheValue, cacheHit := c.getFromInternalCache(key)
		if cacheHit {
			return cacheValue, nil
		}

		start := time.Now()
		expireTime := start.Add(c.expireAfterLoadStart)
		value, err := c.loader(key)

		if err != nil {
			// TODO: how to handle when loader returns an error? (retryable?)
			// TODO: should this error be wrapped?
			return nil, err
		}

		ttl := expireTime.Sub(time.Now())
		if ttl > 0 {
			c.cache.SetWithTTL(key, value, 1, ttl)
		}

		return &value, nil
	})

	return value, err
}

func (c *LoadingCache[K, V]) getFromInternalCache(key K) (*V, bool) {
	value, cacheHit := c.cache.Get(key)
	if cacheHit {
		return value.(*V), true
	} else {
		return nil, false
	}
}
