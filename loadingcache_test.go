package loadingcache_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ichyo/loadingcache"
)

func TestSingleLoad(t *testing.T) {
	cache, err := loadingcache.NewCache(&loadingcache.Config[string, string]{
		MaxItems: 100,
		Loader: func(s string) (string, error) {
			return s + "!", nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from NewCache: %v", err)
	}
	expected := "foo!"
	actual, err := cache.Get("foo")
	if err != nil {
		t.Fatalf("unexpected error from Get: %v", err)
	}
	if *actual != expected {
		t.Fatalf("expected to get %q but %q", expected, *actual)
	}
}

func TestLoadError(t *testing.T) {
	loadError := errors.New("failing for test")
	cache, err := loadingcache.NewCache(&loadingcache.Config[string, *string]{
		MaxItems: 100,
		Loader: func(s string) (*string, error) {
			return nil, loadError
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from NewCache: %v", err)
	}
	value, err := cache.Get("foo")

	if value != nil {
		t.Fatalf("unexpected value from Get: %v", *value)
	}
	if err != loadError {
		t.Fatalf("expected %v but %v", loadError, err)
	}
}

func TestLoadPanic(t *testing.T) {
	cache, err := loadingcache.NewCache(&loadingcache.Config[string, *string]{
		MaxItems: 100,
		Loader: func(s string) (*string, error) {
			panic("panic for test")
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from NewCache: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("cache.Get didn't panic though loader panicked")
		}
	}()
	_, err = cache.Get("foo")
	t.Logf("returned err is: %+v", err)
}

func TestLoadOnlyOnce(t *testing.T) {
	var call int32

	cache, err := loadingcache.NewCache(&loadingcache.Config[string, string]{
		MaxItems: 10,
		Loader: func(s string) (string, error) {
			atomic.AddInt32(&call, 1)
			return s + "!", nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error from NewCache: %v", err)
	}

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			cache.Get("foo")
			wg.Done()
		}()
	}

	wg.Wait()

	if call != 1 {
		t.Fatalf("expected 1 loader call but %d", call)
	}
}

func TestLoadOnlyTwiceWithDelay(t *testing.T) {
	var call int32

	cache, err := loadingcache.NewCache(&loadingcache.Config[string, string]{
		MaxItems: 10,
		Loader: func(s string) (string, error) {
			atomic.AddInt32(&call, 1)
			return s + "!", nil
		},
		ExpireAfterLoadStart: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error from NewCache: %v", err)
	}

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			cache.Get("foo")
			wg.Done()
		}()
	}

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			cache.Get("foo")
			wg.Done()
		}()
	}

	wg.Wait()

	if call != 2 {
		t.Fatalf("expected 2 loader calls but %d", call)
	}
}
