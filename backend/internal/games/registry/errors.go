package registry

import "errors"

var (
	ErrDriverAlreadyRegistered = errors.New("driver already registered")
	ErrDriverNotFound          = errors.New("driver not found")
	ErrInvalidDriver           = errors.New("invalid driver")
	ErrNotImplemented          = errors.New("not implemented")
)

type DriverInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
