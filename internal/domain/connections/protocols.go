package connections

// SupportedProtocols is the protocol set represented by the product
// foundation. It is metadata only; transport implementations stay elsewhere.
var SupportedProtocols = []Protocol{
	ProtocolSSH,
	ProtocolSFTP,
	ProtocolRDP,
	ProtocolLocal,
}
