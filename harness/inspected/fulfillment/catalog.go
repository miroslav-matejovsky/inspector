package fulfillment

// SKU identifies a product, for example "sku-001".
type SKU string

// Product is a sellable item.
type Product struct {
	SKU          SKU
	Name         string
	PriceCents   int64
	Stock        int // units on hand
	Capacity     int // stock level after a restock
	ReorderPoint int // a restock happens when Stock <= ReorderPoint
}

// StandardCatalog returns the 5 products of the simulated shop. Every call
// returns a new slice.
func StandardCatalog() []Product {
	return []Product{
		{SKU: "sku-001", Name: "Mechanical Keyboard", PriceCents: 8900, Stock: 40, Capacity: 40, ReorderPoint: 10},
		{SKU: "sku-002", Name: "USB-C Dock", PriceCents: 14900, Stock: 20, Capacity: 20, ReorderPoint: 5},
		{SKU: "sku-003", Name: "27in Monitor", PriceCents: 32900, Stock: 10, Capacity: 10, ReorderPoint: 3},
		{SKU: "sku-004", Name: "Webcam", PriceCents: 5900, Stock: 30, Capacity: 30, ReorderPoint: 8},
		{SKU: "sku-005", Name: "Desk Lamp", PriceCents: 3900, Stock: 50, Capacity: 50, ReorderPoint: 10},
	}
}
