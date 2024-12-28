// Package monads TODO this module can be move to one domain lib
package monads

// Optional is type safe struct to specify optional values
// this type help create idiomatic code, with specify detailed code semantic.
// For example next code:
//
//	buildSession(previousSessionID string, sessionData io.Writer) error {
//	    sessionID = previousSessionID
//	    if sessionID == "" {
//	       sessionID = uuid.NewString()
//	    }
//	    _, err := sessionData.Write([]byte(sessionID))
//	    return err
//	}
//
// have problem in sessionID check. What if empty session string is correct value? This empty string
// transferred to function from request headers or jwt token, or var string specified as bug?
// next code remove this questions:
//
//	buildSession(previousSessionID monads.Optional[string], sessionData io.Writer) error {
//	    sessionID = previousSessionID.Value
//	    if  previousSessionID.IsEmpty() {
//	       sessionID = uuid.NewString()
//	    }
//	    _, err := sessionData.Write([]byte(sessionID))
//	    return err
//	}
//
// for creation this type need use constructor, all other ways create empty optional type because
// isNotEmpty field is private and by default created as false.
type Optional[T any] struct {
	Value      T
	isNotEmpty bool
}

// OptionalOf create valued optional type have not empty criteria and contain Value field.
func OptionalOf[T any](value T) Optional[T] {
	return Optional[T]{
		Value:      value,
		isNotEmpty: true,
	}
}

// EmptyOf create empty optional type with empty value.
func EmptyOf[T any]() Optional[T] {
	return Optional[T]{}
}

// IsEmpty give information about state of optional variable.
func (opt Optional[T]) IsEmpty() bool {
	return !opt.isNotEmpty
}
