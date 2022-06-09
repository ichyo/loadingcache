package loadingcache

import "testing"

func TestSingleLoad(t *testing.T) {
	cache, err := NewCache(&Config[string, string]{
		MaxItems: 10,
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
