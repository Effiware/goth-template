package hda

// Sentinel errors a ViewHandlerT returns to pick its HTTP status; anything else
// is a 500. WithHTMLFallback maps them, and only these are logged at Warn.

// UnauthorizedError: not authenticated.
type UnauthorizedError struct{ Message string }

func (e *UnauthorizedError) Error() string { return e.Message }

// ForbiddenError: authenticated but not allowed.
type ForbiddenError struct{ Message string }

func (e *ForbiddenError) Error() string { return e.Message }

// NotFoundError: absent, or invisible to this caller.
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// ConflictError: lost an optimistic lock, or a duplicate.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }
