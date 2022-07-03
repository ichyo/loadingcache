# LoadingCache

A simple golang implementation of LoadingCache in Guava library.

## How to use

```golang
cache, err := loadingcache.NewCache(&loadingcache.Config[string, string]{
    MaxItems: 10000,
    Loader: func(key string) (string, error) {
        return doSomeSlowCalculation(key)
    },
    ExpireAfterLoadStart: 100 * time.Millisecond, // Optional
})


value, err := cache.Get("foo")
```
