package arena

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/redisx"
)

func TestEloStoreApplyResultUpdatesRatings(t *testing.T) {
	r := redisx.NewTest(t)
	store := EloStore{R: r, K: 32}
	a, b := uuid.New(), uuid.New()
	ctx := context.Background()

	da, db, err := store.ApplyResult(ctx, uuid.New(), a, b, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	if da <= 0 || db >= 0 {
		t.Fatalf("winner should gain, loser lose: da=%d db=%d", da, db)
	}
	ra, _ := store.Rating(ctx, a)
	rb, _ := store.Rating(ctx, b)
	if ra != 1000+da || rb != 1000+db {
		t.Fatalf("ratings ra=%d rb=%d", ra, rb)
	}
	if MedalForRating(ra) == "" {
		t.Fatal("medal empty")
	}
}

func TestEloStoreApplyResultIsIdempotentPerMatch(t *testing.T) {
	r := redisx.NewTest(t)
	store := EloStore{R: r, K: 32}
	a, b, matchID := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()

	da1, db1, err := store.ApplyResult(ctx, matchID, a, b, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	ra1, _ := store.Rating(ctx, a)
	rb1, _ := store.Rating(ctx, b)
	da2, db2, err := store.ApplyResult(ctx, matchID, a, b, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	ra2, _ := store.Rating(ctx, a)
	rb2, _ := store.Rating(ctx, b)
	if da1 != da2 || db1 != db2 || ra1 != ra2 || rb1 != rb2 {
		t.Fatalf("retry changed ELO: delta %d/%d -> %d/%d rating %d/%d -> %d/%d", da1, db1, da2, db2, ra1, rb1, ra2, rb2)
	}
}

func TestEloStoreDrawNoHugeSwing(t *testing.T) {
	r := redisx.NewTest(t)
	store := EloStore{R: r, K: 32}
	a, b := uuid.New(), uuid.New()
	ctx := context.Background()
	da, db, err := store.ApplyResult(ctx, uuid.New(), a, b, 50, 50)
	if err != nil {
		t.Fatal(err)
	}
	if da != 0 || db != 0 {
		t.Fatalf("equal ratings draw should be 0/0, got %d/%d", da, db)
	}
}

func TestForfeitOutcomeLost(t *testing.T) {
	outA, outB := OutcomeFromScores(10, 5)
	if outA != "won" || outB != "lost" {
		t.Fatalf("%s/%s", outA, outB)
	}
	// Forfeit path forces quitter lost regardless of score — covered in match.finish.
	if MedalForRating(1000) != "bronze" || MedalForRating(2000) != "brilliant" {
		t.Fatal("medal tiers")
	}
}
