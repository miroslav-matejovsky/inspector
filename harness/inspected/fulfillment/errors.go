package fulfillment

import "errors"

// Business validation errors. Returned errors wrap these with context, so
// callers match with errors.Is.
var (
	ErrInvalidCatalog        = errors.New("invalid catalog")
	ErrUnknownProduct        = errors.New("unknown product")
	ErrInvalidQuantity       = errors.New("invalid quantity")
	ErrInvalidChannel        = errors.New("invalid channel")
	ErrUnknownDependency     = errors.New("unknown dependency")
	ErrInvalidDependencyMode = errors.New("invalid dependency mode")
	ErrInvalidOrderStatus    = errors.New("invalid order status")
)
