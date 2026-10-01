package cart

import "strings"

type Cart struct{ items []string }

func New() *Cart { return &Cart{} }

func (c *Cart) Add(item string) {
	c.items = append(c.items, strings.TrimSpace(item))
}

func (c Cart) Len() int { return len(c.items) }

func (c Cart) Full() bool { return len(c.items) >= 3 }
