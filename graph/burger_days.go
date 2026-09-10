package graph

import (
	"errors"
	"fmt"
	"graphql-go/auth"
	"graphql-go/core/schedule"
	"graphql-go/graph/model"
	"graphql-go/persistence"

	"gorm.io/gorm"
)

// maxBulkDeleteBurgerDays caps how many ids delete_burger_days accepts in one call,
// so a single request cannot open an unbounded number of transactions.
const maxBulkDeleteBurgerDays = 100

// hydrateBurgerDays fills in the author and orders-count of every day in one query
// each, instead of one query per day. Call it from any resolver that returns a list
// of burger days; the per-field resolvers fall back to their own query when a day
// was not hydrated.
func hydrateBurgerDays(db *gorm.DB, days []*model.BurgerDay) error {
	if len(days) == 0 {
		return nil
	}

	dayIDs := make([]string, 0, len(days))
	authorIDs := make([]string, 0, len(days))
	seenAuthor := make(map[string]struct{}, len(days))
	for _, day := range days {
		dayIDs = append(dayIDs, day.ID)
		if day.AuthorId == "" {
			continue
		}
		if _, ok := seenAuthor[day.AuthorId]; !ok {
			seenAuthor[day.AuthorId] = struct{}{}
			authorIDs = append(authorIDs, day.AuthorId)
		}
	}

	// One grouped count for every day, rather than a count per day.
	type countRow struct {
		BurgerDayID string
		Total       int
	}
	var countRows []countRow
	if err := db.Model(&persistence.Order{}).
		Select("burger_day_id, count(*) as total").
		Where("burger_day_id IN ?", dayIDs).
		Group("burger_day_id").
		Find(&countRows).Error; err != nil {
		return err
	}
	counts := make(map[string]int, len(countRows))
	for _, row := range countRows {
		counts[row.BurgerDayID] = row.Total
	}

	// One fetch for every distinct author.
	authors := make(map[string]*model.User, len(authorIDs))
	if len(authorIDs) > 0 {
		var users []*persistence.User
		if err := db.Where("id IN ?", authorIDs).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			authors[user.ID] = persistence.UserToModel(user)
		}
	}

	for _, day := range days {
		count := counts[day.ID] // absent means the day has no orders
		day.OrdersCountCache = &count
		if author, ok := authors[day.AuthorId]; ok {
			day.Author = author
		}
	}

	return nil
}

// deleteBurgerDay removes a burger day and its orders in one transaction, so a
// partial failure never leaves orders behind pointing at a day that is gone.
// Refusals are returned as a result with deleted = false, never as an error.
func deleteBurgerDay(db *gorm.DB, user *persistence.User, burgerDayID string) *model.DeleteBurgerDayResult {
	result := &model.DeleteBurgerDayResult{ID: burgerDayID}

	if user == nil {
		return refused(result, "You must be logged in to delete a burger day")
	}

	ordersDeleted := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		var day persistence.BurgerDay
		if err := tx.First(&day, "id = ?", burgerDayID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errBurgerDayRefused{"No burger day with that id exists"}
			}
			return err
		}

		if !canDeleteBurgerDay(user, &day) {
			return errBurgerDayRefused{"Only the author or an admin can delete this burger day"}
		}

		res := tx.Where("burger_day_id = ?", day.ID).Delete(&persistence.Order{})
		if res.Error != nil {
			return res.Error
		}
		ordersDeleted = int(res.RowsAffected)

		return tx.Delete(&persistence.BurgerDay{ID: day.ID}).Error
	})

	var refusal errBurgerDayRefused
	switch {
	case errors.As(err, &refusal):
		return refused(result, refusal.reason)
	case err != nil:
		return refused(result, fmt.Sprintf("Could not delete this burger day: %s", err.Error()))
	}

	result.Deleted = true
	result.OrdersDeleted = ordersDeleted
	return result
}

// canDeleteBurgerDay encodes the authorization rule: the author of a burger day may
// delete it, and admins may delete any burger day. See auth.IsAdmin.
func canDeleteBurgerDay(user *persistence.User, day *persistence.BurgerDay) bool {
	return day.AuthorId == user.ID || auth.IsAdmin(user.Email)
}

// errBurgerDayRefused aborts the delete transaction with a reason meant for the user.
type errBurgerDayRefused struct{ reason string }

func (e errBurgerDayRefused) Error() string { return e.reason }

func refused(result *model.DeleteBurgerDayResult, reason string) *model.DeleteBurgerDayResult {
	result.Deleted = false
	result.OrdersDeleted = 0
	result.Error = &reason
	return result
}

// offScheduleBurgerDayIDs returns the ids of every burger day not held on a Thursday.
func offScheduleBurgerDayIDs(db *gorm.DB) ([]string, error) {
	var days []struct {
		ID   string
		Date string
	}
	if err := db.Model(&persistence.BurgerDay{}).Select("id, date").Find(&days).Error; err != nil {
		return nil, err
	}

	var ids []string
	for _, day := range days {
		if schedule.IsOffSchedule(day.Date) {
			ids = append(ids, day.ID)
		}
	}
	return ids, nil
}
