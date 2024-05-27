package errors

import "errors"

var ServerExists = errors.New("Server with this address and port already exists in database")