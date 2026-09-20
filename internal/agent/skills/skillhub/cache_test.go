package skillhub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeClock is an injectable clock for TTL tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1700000000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// scriptedInner is a Client double whose responses are queued per call.
type scriptedInner struct {
	mu        sync.Mutex
	searches  int
	rankings  int
	downloads int

	searchResp   []SkillSummary
	searchErr    error
	rankingsResp []SkillSummary
	rankingsErr  error
	downloadResp []byte
	downloadErr  error

	blockSearch chan struct{} // when non-nil, each Search waits for a close
}

func (f *scriptedInner) Search(ctx context.Context, query string, limit int) ([]SkillSummary, error) {
	f.mu.Lock()
	f.searches++
	resp, err, block := f.searchResp, f.searchErr, f.blockSearch
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	return resp, err
}

func (f *scriptedInner) Rankings(ctx context.Context, kind string) ([]SkillSummary, error) {
	f.mu.Lock()
	f.rankings++
	resp, err := f.rankingsResp, f.rankingsErr
	f.mu.Unlock()
	return resp, err
}

func (f *scriptedInner) Download(ctx context.Context, slug string) ([]byte, error) {
	f.mu.Lock()
	f.downloads++
	resp, err := f.downloadResp, f.downloadErr
	f.mu.Unlock()
	return resp, err
}

func (f *scriptedInner) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.searches, f.rankings, f.downloads
}

var goodResults = []SkillSummary{{Slug: "a", Name: "A", Description: "d", Version: "1"}}

func TestCachedFreshHitAvoidsRefetch(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	for i := 0; i < 5; i++ {
		results, err := c.Search(context.Background(), "q", 50)
		require.NoError(t, err)
		require.Equal(t, goodResults, results)
	}
	s, _, _ := inner.counts()
	require.Equal(t, 1, s)
}

func TestCachedTTLExpiryTriggersRefetch(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	_, err := c.Search(context.Background(), "q", 50)
	require.NoError(t, err)

	clock.advance(299 * time.Second)
	_, err = c.Search(context.Background(), "q", 50)
	require.NoError(t, err)
	s, _, _ := inner.counts()
	require.Equal(t, 1, s, "still inside TTL: no refetch")

	clock.advance(2 * time.Second)
	_, err = c.Search(context.Background(), "q", 50)
	require.NoError(t, err)
	s, _, _ = inner.counts()
	require.Equal(t, 2, s, "past TTL: refetch")
}

func TestCachedKeysAreScopedPerOperationAndArgument(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults, downloadResp: []byte("zip")}
	clock := newFakeClock()
	c := newCached(inner, time.Hour, clock.Now)

	_, err := c.Search(context.Background(), "q", 50)
	require.NoError(t, err)
	_, err = c.Search(context.Background(), "other", 50)
	require.NoError(t, err)
	_, err = c.Download(context.Background(), "q") // same string, different op
	require.NoError(t, err)
	_, err = c.Download(context.Background(), "q")
	require.NoError(t, err)

	s, _, d := inner.counts()
	require.Equal(t, 2, s)
	require.Equal(t, 1, d)
}

func TestCachedSingleflightDeduplicatesConcurrentMisses(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults, blockSearch: make(chan struct{})}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	const callers = 16
	type outcome struct {
		results []SkillSummary
		err     error
	}
	outcomes := make(chan outcome, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results, err := c.Search(context.Background(), "q", 50)
			outcomes <- outcome{results: results, err: err}
		}()
	}
	close(start)
	time.Sleep(150 * time.Millisecond) // let every caller pile onto the flight
	close(inner.blockSearch)           // release the single upstream fetch
	wg.Wait()
	close(outcomes)
	for o := range outcomes {
		require.NoError(t, o.err)
		require.Equal(t, goodResults, o.results)
	}

	s, _, _ := inner.counts()
	require.Equal(t, 1, s, "one upstream fetch for all concurrent callers")
}

func TestCachedStaleServedWithMarkerOnInnerError(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	_, err := c.Search(context.Background(), "q", 50)
	require.NoError(t, err)

	inner.mu.Lock()
	inner.searchErr = fmt.Errorf("%w: HTTP 503", ErrMarket)
	inner.mu.Unlock()
	clock.advance(301 * time.Second)

	results, err := c.Search(context.Background(), "q", 50)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrStaleOnly)
	require.Equal(t, goodResults, results, "stale value must be served alongside the marker error")
	require.NotErrorIs(t, err, ErrUnreachable)
}

func TestCachedUnreachableWhenNoValueCached(t *testing.T) {
	inner := &scriptedInner{searchErr: fmt.Errorf("%w: HTTP 500", ErrMarket)}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	results, err := c.Search(context.Background(), "q", 50)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnreachable)
	require.ErrorIs(t, err, ErrMarket, "underlying error chain must survive the wrap")
	require.Nil(t, results)
	require.NotErrorIs(t, err, ErrStaleOnly)
}

func TestCachedRecoversAfterStaleWindow(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults}
	clock := newFakeClock()
	c := newCached(inner, 300*time.Second, clock.Now)

	_, err := c.Search(context.Background(), "q", 50)
	require.NoError(t, err)

	fresh := []SkillSummary{{Slug: "b", Name: "B"}}
	inner.mu.Lock()
	inner.searchErr = fmt.Errorf("%w: HTTP 503", ErrMarket)
	inner.mu.Unlock()
	clock.advance(301 * time.Second)
	_, err = c.Search(context.Background(), "q", 50)
	require.ErrorIs(t, err, ErrStaleOnly)

	inner.mu.Lock()
	inner.searchErr = nil
	inner.searchResp = fresh
	inner.mu.Unlock()
	results, err := c.Search(context.Background(), "q", 50)
	require.NoError(t, err)
	require.Equal(t, fresh, results)
}

func TestCachedDownloadStaleAndUnreachable(t *testing.T) {
	inner := &scriptedInner{downloadResp: []byte("zip")}
	clock := newFakeClock()
	c := newCached(inner, time.Minute, clock.Now)

	got, err := c.Download(context.Background(), "slug")
	require.NoError(t, err)
	require.Equal(t, []byte("zip"), got)

	inner.mu.Lock()
	inner.downloadErr = errors.New("connection reset")
	inner.mu.Unlock()
	clock.advance(2 * time.Minute)

	got, err = c.Download(context.Background(), "slug")
	require.ErrorIs(t, err, ErrStaleOnly)
	require.Equal(t, []byte("zip"), got)
}

func TestCachedRankingsInvalidKindShortCircuits(t *testing.T) {
	inner := &scriptedInner{}
	clock := newFakeClock()
	c := newCached(inner, time.Minute, clock.Now)

	_, err := c.Rankings(context.Background(), "nope")
	require.ErrorIs(t, err, ErrUnsupportedRanking)
	_, r, _ := inner.counts()
	require.Equal(t, 0, r)
}

func TestCachedRankingsFreshHit(t *testing.T) {
	inner := &scriptedInner{rankingsResp: goodResults}
	clock := newFakeClock()
	c := newCached(inner, time.Minute, clock.Now)

	for i := 0; i < 3; i++ {
		results, err := c.Rankings(context.Background(), "hot")
		require.NoError(t, err)
		require.Equal(t, goodResults, results)
	}
	_, r, _ := inner.counts()
	require.Equal(t, 1, r)
}

func TestNewCachedConstructor(t *testing.T) {
	inner := &scriptedInner{searchResp: goodResults}
	c := NewCached(inner, time.Minute)
	require.NotNil(t, c)
	var _ Client = c
}
