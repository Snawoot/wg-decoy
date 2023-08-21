package client

type Config struct {
	LocalPort     uint16
	RemoteAddress string
	ClientReq     []byte
	ServerResp    []byte
}
