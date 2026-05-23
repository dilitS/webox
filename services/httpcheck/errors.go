package httpcheck

import "errors"

const hoursPerDay = 24

var errNoPeerCertificates = errors.New("httpcheck: TLS peer presented no certificates")
