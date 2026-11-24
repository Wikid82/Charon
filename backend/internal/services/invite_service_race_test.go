package services

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// acceptBarrier holds every accept at the seam until all of them have passed their
// pre-checks, so the writes race for real. A goroutine that never reaches the seam
// makes the others time out, which fails the test instead of hanging it.
type acceptBarrier struct {
	arrivals    sync.WaitGroup
	allArrived  chan struct{}
	timedOut    chan struct{}
	timeoutOnce sync.Once
	timeout     time.Duration
}

func newAcceptBarrier(parties int, timeout time.Duration) *acceptBarrier {
	b := &acceptBarrier{
		allArrived: make(chan struct{}),
		timedOut:   make(chan struct{}),
		timeout:    timeout,
	}
	b.arrivals.Add(parties)
	go func() {
		b.arrivals.Wait()
		close(b.allArrived)
	}()
	return b
}

// wait is the beforeLock seam: it records this arrival and blocks until everyone has arrived.
func (b *acceptBarrier) wait() {
	b.arrivals.Done()
	select {
	case <-b.allArrived:
	case <-time.After(b.timeout):
		b.timeoutOnce.Do(func() { close(b.timedOut) })
	}
}

// TestInviteService_AcceptConsumesTokenOnce sends many accepts with one token at the same
// moment and asserts exactly one of them succeeds. Runs on the default pool and on a single
// connection, for several rounds, with fresh state per round.
func TestInviteService_AcceptConsumesTokenOnce(t *testing.T) {
	const (
		concurrency = 16
		rounds      = 5
		token       = "race-token"
	)

	pools := []struct {
		name     string
		maxConns int
	}{
		{name: "default pool", maxConns: 0},
		{name: "single connection", maxConns: 1},
	}

	for _, pool := range pools {
		t.Run(pool.name, func(t *testing.T) {
			for round := 0; round < rounds; round++ {
				db := openInviteTestDB(t)
				if pool.maxConns > 0 {
					sqlDB, err := db.DB()
					require.NoError(t, err)
					sqlDB.SetMaxOpenConns(pool.maxConns)
				}
				invitee := seedPendingInvite(t, db, fmt.Sprintf("race-%d@example.com", round), token)

				svc := newTestInviteService(db)
				barrier := newAcceptBarrier(concurrency, 5*time.Second)
				svc.beforeLock = barrier.wait

				type outcome struct {
					index int
					err   error
				}
				results := make(chan outcome, concurrency)
				var wg sync.WaitGroup
				for i := 0; i < concurrency; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						_, err := svc.Accept(token, fmt.Sprintf("Racer %d", i), fmt.Sprintf("racer-password-%02d", i), "")
						results <- outcome{index: i, err: err}
					}(i)
				}
				wg.Wait()
				close(results)

				select {
				case <-barrier.timedOut:
					t.Fatalf("round %d: not every accept reached the write; a goroutine failed before the write", round)
				default:
				}

				winners := []int{}
				for res := range results {
					if res.err == nil {
						winners = append(winners, res.index)
						continue
					}
					assert.True(t, errors.Is(res.err, ErrInviteNotFound) || errors.Is(res.err, ErrInviteClosed),
						"round %d: unexpected error for racer %d: %v", round, res.index, res.err)
				}
				require.Len(t, winners, 1, "round %d: exactly one accept must consume the token", round)

				stored := reloadInviteUser(t, db, invitee.ID)
				assert.Equal(t, fmt.Sprintf("Racer %d", winners[0]), stored.Name)
				assert.True(t, (&models.User{PasswordHash: stored.PasswordHash}).CheckPassword(fmt.Sprintf("racer-password-%02d", winners[0])))
				assert.Equal(t, models.InviteStatusAccepted, stored.InviteStatus)
			}
		})
	}
}
