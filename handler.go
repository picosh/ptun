package ptun

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/charmbracelet/ssh"
)

type ctxAddressKey struct{}

func getAddressCtx(ctx ssh.Context) (string, error) {
	address, ok := ctx.Value(ctxAddressKey{}).(string)
	if address == "" || !ok {
		return address, fmt.Errorf("address not set on `ssh.Context()` for connection")
	}
	return address, nil
}
func setAddressCtx(ctx ssh.Context, address string) {
	ctx.SetValue(ctxAddressKey{}, address)
}

type WebTunnelHandler struct {
	HttpHandler HttpHandlerFn
}

func (wt *WebTunnelHandler) GetHttpHandler() HttpHandlerFn {
	return wt.HttpHandler
}

func (wt *WebTunnelHandler) CreateListener(sesh ssh.Session) (net.Listener, error) {
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		log.Println("Unable to create tempfile:", err)
		return nil, err
	}

	tempFile.Close()
	tempFileName := tempFile.Name()
	setAddressCtx(sesh.Context(), tempFileName)
	os.Remove(tempFileName)

	connListener, err := net.Listen("unix", tempFileName)
	if err != nil {
		log.Println("Unable to listen to unix socket:", err)
		return nil, err
	}

	return connListener, nil
}

func (wt *WebTunnelHandler) CreateConn(ctx ssh.Context) (net.Conn, error) {
	address, err := getAddressCtx(ctx)
	if err != nil {
		return nil, err
	}
	return net.Dial("unix", address)
}
