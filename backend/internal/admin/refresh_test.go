package admin

import (
	"errors"
	"sync"
	"testing"

	"avtotest.uz/backend/internal/testdb"
)

func TestRefreshTokenRotationIsCompareAndSwap(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	svc := Service{Store: store, Secret: secret}
	login, err := svc.Login(t.Context(), "ops@example.uz", "password123", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, refreshErr := svc.Refresh(t.Context(), login.Pair.RefreshToken, "", nil)
			errs <- refreshErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var succeeded, rejected int
	for refreshErr := range errs {
		switch {
		case refreshErr == nil:
			succeeded++
		case errors.Is(refreshErr, ErrInvalidCreds):
			rejected++
		default:
			t.Fatalf("unexpected refresh error: %v", refreshErr)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("succeeded=%d rejected=%d want 1/1", succeeded, rejected)
	}
}
