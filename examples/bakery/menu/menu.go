// Package menu reads the bakery's menu.
package menu

import (
	"fmt"

	"bakery/money"

	"github.com/BurntSushi/toml"
)

// Item is one thing the bakery sells.
type Item struct {
	Name  string
	Price money.Cents
}

// Menu is everything for sale, by name.
type Menu map[string]Item

// Parse reads a menu written in TOML:
//
//	[[item]]
//	name = "croissant"
//	price = 350
func Parse(text string) (Menu, error) {
	var file struct{ Item []Item }
	if _, err := toml.Decode(text, &file); err != nil {
		return nil, fmt.Errorf("menu: %w", err)
	}
	m := Menu{}
	for _, it := range file.Item {
		m[it.Name] = it
	}
	return m, nil
}
