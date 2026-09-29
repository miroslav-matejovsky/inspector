package fulfillment

// Snapshot is an immutable copy of a State at one tick. It shares no memory
// with the State it was taken from.
type Snapshot struct {
	Now          Tick
	Products     []Product    // SKU order
	Orders       []Order      // placement order
	Dependencies []Dependency // name order
}

// Product returns the product with the given SKU.
func (s Snapshot) Product(sku SKU) (Product, bool) {
	for _, p := range s.Products {
		if p.SKU == sku {
			return p, true
		}
	}
	return Product{}, false
}

// Order returns the order with the given ID.
func (s Snapshot) Order(id OrderID) (Order, bool) {
	for _, o := range s.Orders {
		if o.ID == id {
			return o, true
		}
	}
	return Order{}, false
}
