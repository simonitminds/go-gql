package graph_test

import (
	"fmt"
	"graphql-go/auth"
	"graphql-go/graph"
	"graphql-go/persistence"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests need a throwaway postgres. Point TEST_DB_URL at one, or run
//
//	createdb gogql_test
//
// against the local server the app already defaults to. They skip when neither
// is reachable, so `go test ./...` stays green without a database.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=changeme dbname=gogql_test"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}

	persistence.EnsureMigrated(db)
	// Start every test from an empty database.
	db.Exec("TRUNCATE orders, burger_days, users CASCADE")

	return db
}

// gqlClient returns a client that runs every request as the given user.
func gqlClient(db *gorm.DB, as *persistence.User) *client.Client {
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: graph.NewResolver(db)}))
	withUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), as)))
	})
	return client.New(withUser)
}

func newUser(t *testing.T, db *gorm.DB, name, email string) *persistence.User {
	t.Helper()
	user := &persistence.User{ID: uuid.New().String(), Name: name, Email: email}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("creating user: %v", err)
	}
	return user
}

func newBurgerDay(t *testing.T, db *gorm.DB, author *persistence.User, date string, orders int) *persistence.BurgerDay {
	t.Helper()
	day := &persistence.BurgerDay{ID: uuid.New().String(), AuthorId: author.ID, Date: date}
	if err := db.Create(day).Error; err != nil {
		t.Fatalf("creating burger day: %v", err)
	}
	for range orders {
		order := &persistence.Order{ID: uuid.New().String(), BurgerDayId: day.ID, UserId: author.ID}
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("creating order: %v", err)
		}
	}
	return day
}

// 2025-02-06 is a Thursday, so these run Thursday, Friday, Saturday, ...
func dateOffset(days int) string {
	return time.Date(2025, 2, 6, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days).Format("2006-01-02")
}

func TestBurgerDaysOffScheduleOnly(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")

	// Two Thursdays and three days that are not.
	newBurgerDay(t, db, author, "2025-02-06", 3) // Thursday
	newBurgerDay(t, db, author, "2025-02-13", 5) // Thursday
	newBurgerDay(t, db, author, "2025-02-05", 1) // Wednesday
	newBurgerDay(t, db, author, "2025-02-08", 0) // Saturday
	newBurgerDay(t, db, author, "2025-02-11", 2) // Tuesday

	var resp struct {
		BurgerDays []struct {
			ID          string
			Date        string
			Weekday     int
			OffSchedule bool
			OrdersCount int
			Author      struct{ ID, Name string }
		} `json:"burger_days"`
	}
	gqlClient(db, author).MustPost(
		`{ burger_days(offScheduleOnly: true) { id date weekday offSchedule ordersCount author { id name } } }`,
		&resp,
	)

	if len(resp.BurgerDays) != 3 {
		t.Fatalf("got %d off-schedule days, want 3", len(resp.BurgerDays))
	}

	wantDates := []string{"2025-02-05", "2025-02-08", "2025-02-11"}
	for i, day := range resp.BurgerDays {
		if day.Date != wantDates[i] {
			t.Errorf("day %d: date = %s, want %s (results must be sorted by date ascending)", i, day.Date, wantDates[i])
		}
		if day.Weekday == 4 {
			t.Errorf("day %s: weekday 4 (Thursday) must not be off-schedule", day.Date)
		}
		if !day.OffSchedule {
			t.Errorf("day %s: offSchedule = false, want true", day.Date)
		}
		if day.Author.Name != "Author" {
			t.Errorf("day %s: author not resolved, got %+v", day.Date, day.Author)
		}
	}

	if resp.BurgerDays[0].OrdersCount != 1 || resp.BurgerDays[1].OrdersCount != 0 || resp.BurgerDays[2].OrdersCount != 2 {
		t.Errorf("orders counts = %d/%d/%d, want 1/0/2",
			resp.BurgerDays[0].OrdersCount, resp.BurgerDays[1].OrdersCount, resp.BurgerDays[2].OrdersCount)
	}

	// Without the filter every day comes back, still sorted by date.
	var all struct {
		BurgerDays []struct{ Date string } `json:"burger_days"`
	}
	gqlClient(db, author).MustPost(`{ burger_days { date } }`, &all)
	if len(all.BurgerDays) != 5 {
		t.Fatalf("got %d days without the filter, want 5", len(all.BurgerDays))
	}
}

// The N+1 is gone when a list of days resolves author and ordersCount: the number of
// queries must not grow with the number of days.
func TestBurgerDaysDoesNotQueryPerRow(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")
	for i := range 60 {
		newBurgerDay(t, db, author, dateOffset(i), 2)
	}

	var queries int
	counted := db.Session(&gorm.Session{NewDB: true})
	if err := counted.Callback().Query().After("*").Register("count_queries", func(*gorm.DB) { queries++ }); err != nil {
		t.Fatalf("registering callback: %v", err)
	}
	if err := counted.Callback().Row().After("*").Register("count_rows", func(*gorm.DB) { queries++ }); err != nil {
		t.Fatalf("registering callback: %v", err)
	}

	var resp struct {
		BurgerDays []struct {
			ID          string
			Date        string
			Weekday     int
			OrdersCount int
			Author      struct{ ID string }
		} `json:"burger_days"`
	}
	start := time.Now()
	gqlClient(counted, author).MustPost(`{ burger_days { id date weekday ordersCount author { id } } }`, &resp)
	elapsed := time.Since(start)

	if len(resp.BurgerDays) != 60 {
		t.Fatalf("got %d days, want 60", len(resp.BurgerDays))
	}
	// The days, the grouped order counts, and the authors: three queries total.
	t.Logf("burger_days issued %d queries for 60 days", queries)
	if queries == 0 {
		t.Fatal("the query counter never fired, so this test proves nothing")
	}
	if queries > 5 {
		t.Errorf("burger_days issued %d queries for 60 days, want a constant handful", queries)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("burger_days took %s for 60 days, want under 500ms", elapsed)
	}
}

func TestDeleteBurgerDaysReportsPerItemResults(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")
	day := newBurgerDay(t, db, author, "2025-02-05", 4)

	var resp struct {
		Results []struct {
			ID            string
			Deleted       bool
			Error         *string
			OrdersDeleted int
		} `json:"delete_burger_days"`
	}
	gqlClient(db, author).MustPost(
		`mutation ($ids: [ID!]!) { delete_burger_days(burgerDayIds: $ids) { id deleted error ordersDeleted } }`,
		&resp,
		client.Var("ids", []string{day.ID, "missing-id"}),
	)

	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want one per requested id", len(resp.Results))
	}

	deleted, missing := resp.Results[0], resp.Results[1]
	if deleted.ID != day.ID || !deleted.Deleted || deleted.Error != nil {
		t.Errorf("first result = %+v, want the day deleted with no error", deleted)
	}
	if deleted.OrdersDeleted != 4 {
		t.Errorf("ordersDeleted = %d, want 4", deleted.OrdersDeleted)
	}
	if missing.ID != "missing-id" || missing.Deleted || missing.Error == nil || *missing.Error == "" {
		t.Errorf("second result = %+v, want deleted:false with a non-empty error", missing)
	}

	// The day's orders went with it rather than being orphaned.
	var orphans int64
	db.Model(&persistence.Order{}).Where("burger_day_id = ?", day.ID).Count(&orphans)
	if orphans != 0 {
		t.Errorf("%d orders survived the deleted burger day", orphans)
	}
}

func TestDeleteBurgerDaysAuthorization(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")
	stranger := newUser(t, db, "Stranger", "stranger@example.com")
	admin := newUser(t, db, "Admin", "seg@it-minds.dk")

	query := `mutation ($ids: [ID!]!) { delete_burger_days(burgerDayIds: $ids) { deleted error } }`
	type result struct {
		Results []struct {
			Deleted bool
			Error   *string
		} `json:"delete_burger_days"`
	}

	// A stranger is refused, as data rather than as a thrown error.
	refusedDay := newBurgerDay(t, db, author, "2025-02-05", 1)
	var refused result
	gqlClient(db, stranger).MustPost(query, &refused, client.Var("ids", []string{refusedDay.ID}))
	if refused.Results[0].Deleted {
		t.Error("a stranger was allowed to delete someone else's burger day")
	}
	if refused.Results[0].Error == nil || *refused.Results[0].Error == "" {
		t.Error("a refused delete must come back with an explanation")
	}

	// The author may delete their own day, and an admin may delete anyone's.
	var byAuthor, byAdmin result
	gqlClient(db, author).MustPost(query, &byAuthor, client.Var("ids", []string{refusedDay.ID}))
	if !byAuthor.Results[0].Deleted {
		t.Errorf("the author could not delete their own day: %+v", byAuthor.Results[0])
	}

	adminDay := newBurgerDay(t, db, author, "2025-02-12", 0)
	gqlClient(db, admin).MustPost(query, &byAdmin, client.Var("ids", []string{adminDay.ID}))
	if !byAdmin.Results[0].Deleted {
		t.Errorf("an admin could not delete another user's day: %+v", byAdmin.Results[0])
	}
}

func TestDeleteBurgerDaysCapsInput(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}

	var resp struct{}
	err := gqlClient(db, author).Post(
		`mutation ($ids: [ID!]!) { delete_burger_days(burgerDayIds: $ids) { id deleted } }`,
		&resp,
		client.Var("ids", ids),
	)
	if err == nil {
		t.Fatal("101 ids were accepted, want a GraphQL error above the 100 cap")
	}
}

func TestBurgerStatsCanIgnoreOffScheduleDays(t *testing.T) {
	db := testDB(t)
	author := newUser(t, db, "Author", "author@example.com")
	newBurgerDay(t, db, author, "2025-02-06", 3) // Thursday
	newBurgerDay(t, db, author, "2025-02-13", 2) // Thursday
	newBurgerDay(t, db, author, "2025-02-05", 4) // Wednesday, a test run

	var resp struct {
		All struct {
			TotalBurgerDays, TotalOrders int
			TopConsumers                 []struct{ TotalOrders, TotalBurgerDays int }
		} `json:"all"`
		Scheduled struct {
			TotalBurgerDays, TotalOrders int
			TopConsumers                 []struct{ TotalOrders, TotalBurgerDays int }
		} `json:"scheduled"`
	}
	gqlClient(db, author).MustPost(`{
		all: burgerStats { totalBurgerDays totalOrders topConsumers { totalOrders totalBurgerDays } }
		scheduled: burgerStats(includeOffSchedule: false) { totalBurgerDays totalOrders topConsumers { totalOrders totalBurgerDays } }
	}`, &resp)

	if resp.All.TotalBurgerDays != 3 || resp.All.TotalOrders != 9 {
		t.Errorf("unfiltered stats = %d days / %d orders, want 3 / 9", resp.All.TotalBurgerDays, resp.All.TotalOrders)
	}
	if resp.Scheduled.TotalBurgerDays != 2 || resp.Scheduled.TotalOrders != 5 {
		t.Errorf("filtered stats = %d days / %d orders, want 2 / 5", resp.Scheduled.TotalBurgerDays, resp.Scheduled.TotalOrders)
	}
	if len(resp.Scheduled.TopConsumers) != 1 || resp.Scheduled.TopConsumers[0].TotalOrders != 5 {
		t.Errorf("topConsumers = %+v, want a single consumer with 5 orders", resp.Scheduled.TopConsumers)
	}
	if resp.Scheduled.TopConsumers[0].TotalBurgerDays != 2 {
		t.Errorf("consumer totalBurgerDays = %d, want 2", resp.Scheduled.TopConsumers[0].TotalBurgerDays)
	}
}

func TestMeIsAdmin(t *testing.T) {
	db := testDB(t)
	admin := newUser(t, db, "Admin", "seg@it-minds.dk")
	regular := newUser(t, db, "Regular", "regular@example.com")

	var resp struct {
		Me struct{ IsAdmin bool } `json:"me"`
	}
	gqlClient(db, admin).MustPost(`{ me { isAdmin } }`, &resp)
	if !resp.Me.IsAdmin {
		t.Error("an admin's me { isAdmin } returned false")
	}
	gqlClient(db, regular).MustPost(`{ me { isAdmin } }`, &resp)
	if resp.Me.IsAdmin {
		t.Error("a regular user's me { isAdmin } returned true")
	}
}
