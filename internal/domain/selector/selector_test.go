package selector

import (
	"sync"
	"testing"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestSingleSelectorReturnsConfiguredResponse(t *testing.T) {
	selected := NewSingle(response("default", 200)).Select()

	assertResponseName(t, selected, "default")
}

func TestSequentialSelectorWrapsAround(t *testing.T) {
	selector := NewSequential([]mapping.Response{response("first", 200), response("second", 201)})

	assertResponseName(t, selector.Select(), "first")
	assertResponseName(t, selector.Select(), "second")
	assertResponseName(t, selector.Select(), "first")
}

func TestSequentialSelectorIsSafeForConcurrentAccess(t *testing.T) {
	selector := NewSequential([]mapping.Response{response("first", 200), response("second", 201)})
	counts := selectConcurrently(selector, 1000)

	if counts["first"] != 500 || counts["second"] != 500 {
		t.Fatalf("expected balanced sequential selections, got %#v", counts)
	}
}

func TestRandomSelectorUsesRandomIndex(t *testing.T) {
	selector := newRandomSelector([]mapping.Response{response("first", 200), response("second", 201)}, fixedIntn(1))

	assertResponseName(t, selector.Select(), "second")
}

func TestWeightedSelectorUsesWeights(t *testing.T) {
	selector := newWeightedSelector([]mapping.Response{weighted("ok", 3), weighted("error", 1)}, cycleIntn(0, 2, 3))

	assertResponseName(t, selector.Select(), "ok")
	assertResponseName(t, selector.Select(), "ok")
	assertResponseName(t, selector.Select(), "error")
}

func selectConcurrently(selector Selector, total int) map[string]int {
	var mu sync.Mutex
	var wg sync.WaitGroup
	counts := map[string]int{}
	for range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selected := selector.Select()
			mu.Lock()
			counts[selected.Name]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return counts
}

func response(name string, status int) mapping.Response {
	return mapping.Response{Name: name, Status: status}
}

func weighted(name string, weight int) mapping.Response {
	return mapping.Response{Name: name, Weight: weight}
}

func fixedIntn(value int) intnFunc {
	return func(int) int { return value }
}

func cycleIntn(values ...int) intnFunc {
	index := 0
	return func(int) int {
		value := values[index%len(values)]
		index++
		return value
	}
}

func assertResponseName(t *testing.T, response mapping.Response, want string) {
	t.Helper()
	if response.Name != want {
		t.Fatalf("expected response %q, got %q", want, response.Name)
	}
}
