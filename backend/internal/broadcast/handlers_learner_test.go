package broadcast

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestLearnerNotificationsIDOR(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)

	owner, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901890001"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901890002"})
	if err != nil {
		t.Fatal(err)
	}
	adminID := insertAdmin(t, pool)

	svc := &Service{Pool: pool, Q: q, Cfg: Config{MaxRecipients: 1000}}
	camp, err := svc.Create(context.Background(), CreateInput{
		AdminID: adminID, Title: "T", Body: "B", Audience: AudienceAllActive,
		Channels: ChannelsInapp, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := svc.ProcessOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		got, _ := svc.Get(context.Background(), camp.ID)
		if got.Status == "completed" || got.Status == "completed_with_errors" {
			break
		}
	}

	list, err := q.ListInappNotifications(context.Background(), sqlc.ListInappNotificationsParams{
		ProfileID: owner.ID,
		Limit:     10,
	})
	if err != nil || len(list) < 1 {
		t.Fatalf("owner notifications=%d err=%v", len(list), err)
	}
	notifID := list[0].ID

	_, err = q.MarkInappNotificationRead(context.Background(), sqlc.MarkInappNotificationReadParams{
		ID:        notifID,
		ProfileID: other.ID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-user mark-read err=%v want ErrNoRows", err)
	}

	row, err := q.MarkInappNotificationRead(context.Background(), sqlc.MarkInappNotificationReadParams{
		ID:        notifID,
		ProfileID: owner.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !row.ReadAt.Valid {
		t.Fatal("expected read_at set")
	}
}
