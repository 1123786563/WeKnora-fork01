package repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestMobileExchangeSingleUse(t *testing.T) {
	db := openRunTestDB(t)
	s := NewMobileExchangeStore(db)
	now := time.Now().UTC()
	require.NoError(t, s.Put(context.Background(), MobileExchange{CodeHash: "h", StateHash: "s", RedirectURI: "weknora://auth-return", Challenge: "p", Subject: "u1", ExpiresAt: now.Add(time.Minute)}))
	_, err := s.ConsumeMobileExchange(context.Background(), "h", "s", "weknora://auth-return", "p", now)
	require.NoError(t, err)
	_, err = s.ConsumeMobileExchange(context.Background(), "h", "s", "weknora://auth-return", "p", now)
	require.ErrorIs(t, err, ErrMobileExchangeInvalid)
}
func TestMobileExchangeRejectsBindingAndExpiry(t *testing.T) {
	db := openRunTestDB(t)
	s := NewMobileExchangeStore(db)
	now := time.Now().UTC()
	require.NoError(t, s.Put(context.Background(), MobileExchange{CodeHash: "h2", StateHash: "s2", RedirectURI: "weknora://auth-return", Challenge: "p2", Subject: "u2", ExpiresAt: now.Add(-time.Second)}))
	for _, in := range [][4]string{{"h2", "s2", "weknora://other", "p2"}, {"h2", "bad", "weknora://auth-return", "p2"}, {"h2", "s2", "weknora://auth-return", "bad"}, {"h2", "s2", "weknora://auth-return", "p2"}} {
		_, err := s.ConsumeMobileExchange(context.Background(), in[0], in[1], in[2], in[3], now)
		require.ErrorIs(t, err, ErrMobileExchangeInvalid)
	}
}
func TestMobileExchangeConcurrentSingleWinner(t *testing.T) {
	db := openRunTestDB(t)
	s := NewMobileExchangeStore(db)
	now := time.Now().UTC()
	require.NoError(t, s.Put(context.Background(), MobileExchange{CodeHash: "hc", StateHash: "sc", RedirectURI: "weknora://auth-return", Challenge: "pc", Subject: "u3", ExpiresAt: now.Add(time.Minute)}))
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ConsumeMobileExchange(context.Background(), "hc", "sc", "weknora://auth-return", "pc", now); err == nil {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, winners)
}
