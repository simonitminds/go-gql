// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"fmt"
	"io"
	"strconv"
)

type AccumulatedOrderLine struct {
	Amount         int             `json:"amount"`
	SpecialRequest []SpecialOrders `json:"specialRequest"`
}

type AccumulatedOrders struct {
	Count   int                     `json:"count"`
	ID      string                  `json:"id"`
	ToOrder []*AccumulatedOrderLine `json:"to_order"`
}

type BurgerStats struct {
	TopConsumers    []*Consumer `json:"topConsumers"`
	TotalBurgerDays int         `json:"totalBurgerDays"`
	TotalOrders     int         `json:"totalOrders"`
}

type Consumer struct {
	TotalBurgerDays int   `json:"totalBurgerDays"`
	TotalOrders     int   `json:"totalOrders"`
	User            *User `json:"user"`
}

type Mutation struct {
}

type Query struct {
}

type User struct {
	Email       string  `json:"email"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	PhoneNumber *string `json:"phoneNumber,omitempty"`
}

type SpecialOrders string

const (
	SpecialOrdersChiliMayo       SpecialOrders = "chili_mayo"
	SpecialOrdersGarlicMayo      SpecialOrders = "garlic_mayo"
	SpecialOrdersGlutenFreeBun   SpecialOrders = "gluten_free_bun"
	SpecialOrdersNoBacon         SpecialOrders = "no_bacon"
	SpecialOrdersNoCheese        SpecialOrders = "no_cheese"
	SpecialOrdersNoSalat         SpecialOrders = "no_salat"
	SpecialOrdersVegetarianPatty SpecialOrders = "vegetarian_patty"
)

var AllSpecialOrders = []SpecialOrders{
	SpecialOrdersChiliMayo,
	SpecialOrdersGarlicMayo,
	SpecialOrdersGlutenFreeBun,
	SpecialOrdersNoBacon,
	SpecialOrdersNoCheese,
	SpecialOrdersNoSalat,
	SpecialOrdersVegetarianPatty,
}

func (e SpecialOrders) IsValid() bool {
	switch e {
	case SpecialOrdersChiliMayo, SpecialOrdersGarlicMayo, SpecialOrdersGlutenFreeBun, SpecialOrdersNoBacon, SpecialOrdersNoCheese, SpecialOrdersNoSalat, SpecialOrdersVegetarianPatty:
		return true
	}
	return false
}

func (e SpecialOrders) String() string {
	return string(e)
}

func (e *SpecialOrders) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SpecialOrders(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid SpecialOrders", str)
	}
	return nil
}

func (e SpecialOrders) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
