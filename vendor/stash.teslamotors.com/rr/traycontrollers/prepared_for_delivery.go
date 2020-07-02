package traycontrollers

// PreparedForDelivery is the message sent from C/D controller to Tower Controller to reserve a fixture while a
// tray is on the way
type PreparedForDelivery struct {
	Tray    string `json:"tray"`
	Fixture string `json:"fixture"`
}
