package profile

// Engine identifies the protocol engine used to establish a connection.
type Engine string

const (
	EngineXray      Engine = "xray"
	EngineAmneziaWG Engine = "amneziawg"
)

// Profile is the normalized internal VPN connection model.
//
// Subscription-specific formats should be parsed into this model before any
// networking plan is created.
type Profile struct {
	ID          string
	Name        string
	Engine      Engine
	Server      string
	Port        uint16
	Protocol    string
	Transport   string
	Security    string
	Fingerprint string
}
