package errs

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownDeviceAction = errors.New("unknown device action")
)

var (
	ErrInvalidTransitionType  = errors.New("invalid transition type")
	ErrTransitionNotSupported = errors.New("transition not supported")
)

var (
	ErrDatabaseAlreadyExists = errors.New("database already exists")
)

var (
	ErrSplitBrain      = fmt.Errorf("split-brain")
	ErrPrimaryNotFound = fmt.Errorf("primary not found")
	ErrAPIError        = fmt.Errorf("api error")
)
