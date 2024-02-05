package persistence

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"graphql-go/graph/model"
	"os"
)

// Assuming the model structs are as defined in your question.
// Note: For GORM, you need to adjust struct tags slightly to match GORM's expectations.

func ConnectGORM() *gorm.DB {
	// Check if the DB_URL environment variable is set.
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		// Use hardcoded values for local development.
		dsn = "host=localhost user=postgres password=changeme dbname=gogql"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}

	EnsureMigrated(db)

	return db
}

func EnsureMigrated(db *gorm.DB) {
	// Automigrate tables
	if err := db.AutoMigrate(&User{}, &BurgerDay{}, &Order{}); err != nil {
		panic(err)
	}
}

func UserToModel(user *User) *model.User {
	return &model.User{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}
}
func UsersToModels(users []*User) []*model.User {
	models := make([]*model.User, len(users))
	for i, user := range users {
		models[i] = UserToModel(user)
	}
	return models
}

func BurgerDayToModel(burgerDay *BurgerDay) *model.BurgerDay {
	return &model.BurgerDay{
		ID:       burgerDay.ID,
		Date:     burgerDay.Date,
		AuthorId: burgerDay.AuthorId,
	}
}

func BurgerDaysToModels(burgerDays []*BurgerDay) []*model.BurgerDay {
	models := make([]*model.BurgerDay, len(burgerDays))
	for i, burgerDay := range burgerDays {
		models[i] = BurgerDayToModel(burgerDay)
	}
	return models
}

func StringsToSpecialOrders(strings []string) ([]model.SpecialOrders, error) {
	specialOrders := make([]model.SpecialOrders, len(strings))
	for i, s := range strings {
		found := false
		for _, validOrder := range model.AllSpecialOrders {
			if string(validOrder) == s {
				specialOrders[i] = validOrder
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid special order: %s", s)
		}
	}
	return specialOrders, nil
}

func SpecialOrdersToStrings(specialOrders []model.SpecialOrders) []string {
	stringsToFormat := make([]string, len(specialOrders))
	for i, order := range specialOrders {
		stringsToFormat[i] = string(order)
	}
	return stringsToFormat
}

func OrderToModel(order *Order) *model.Order {
	special, _ := StringsToSpecialOrders(order.SpecialRequest)
	return &model.Order{
		ID:             order.ID,
		BurgerDayId:    order.BurgerDayId,
		UserId:         order.UserId,
		SpecialRequest: special,
	}
}

func OrdersToModels(orders []*Order) []*model.Order {
	models := make([]*model.Order, len(orders))
	for i, order := range orders {
		models[i] = OrderToModel(order)
	}
	return models
}

// Model definitions should be updated to use GORM struct tags for primary keys,
// foreign keys, and any other constraints or indexes as needed.

type BurgerDay struct {
	ID       string   `gorm:"primaryKey" json:"id"`
	AuthorId string   `json:"authorId"`
	Author   *User    `gorm:"foreignKey:AuthorId" json:"author"`
	Date     string   `gorm:"unique;type:text" json:"date"`
	Orders   []*Order `gorm:"foreignKey:BurgerDayId" json:"orders"`
}

type User struct {
	ID         string       `gorm:"primaryKey" json:"id"`
	Name       string       `json:"name"`
	Email      string       `json:"email"`
	BurgerDays []*BurgerDay `gorm:"foreignKey:AuthorId" json:"burgerDays"`
	Orders     []*Order     `gorm:"foreignKey:UserId" json:"orders"`
}

type Order struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	BurgerDayId    string     `json:"burgerDayId"`
	BurgerDay      *BurgerDay `gorm:"foreignKey:BurgerDayId" json:"BurgerDay"`
	UserId         string     `json:"userId"`
	User           *User      `gorm:"foreignKey:UserId" json:"user"`
	SpecialRequest []string   `gorm:"type:text[]" json:"specialRequest"`
}
