package transport

// UploadPayload is the encrypted request payload shared between client and server.
type UploadPayload struct {
	Filename string
	Data     []byte
	PinLink  *string
}
