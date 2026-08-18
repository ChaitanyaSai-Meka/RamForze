package handshake

const (
	HandshakeStatusConnected                = "CONNECTED"
	HandshakeStatusRejectedUnsupported      = "REJECTED_UNSUPPORTED_VERSION"
	HandshakeStatusRejectedInvalidFields    = "REJECTED_INVALID_FIELDS"
	HandshakeStatusRejectedUnauthorized     = "REJECTED_UNAUTHORIZED"
	HandshakeStatusRejectedNoPortsAvailable = "REJECTED_NO_PORTS"
	HandshakeStatusRejectedNonceUsed		= "REJECTED_NONCE_USED"
	HandshakeStatusRejectedRateLimit		= "REJECTED_RATE_LIMIT"
)
