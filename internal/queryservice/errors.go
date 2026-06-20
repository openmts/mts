package queryservice

import "errors"

var ErrAdmissionRejected = errors.New("query admission rejected")

var ErrQueueFull = errors.New("query queue full")

var ErrStreamingUnsupported = errors.New("query streaming unsupported")

var ErrUnauthorized = errors.New("query unauthorized")

var ErrUnsupportedQueryLanguage = errors.New("query language unsupported")

var ErrDistributedUnsupported = errors.New("distributed query unsupported")
