package ptun

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type forwardedTCPPayload struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

type LocalForwardFn = func(*ssh.Server, *gossh.ServerConn, gossh.NewChannel, ssh.Context)
type HttpHandlerFn = func(ctx ssh.Context) http.Handler

type WebTunnel interface {
	GetHttpHandler() HttpHandlerFn
	CreateListener(ctx ssh.Context) (net.Listener, error)
	CreateConn(ctx ssh.Context) (net.Conn, error)
}

func ErrorHandler(sesh ssh.Session, err error) {
	_, _ = fmt.Fprint(sesh.Stderr(), err, "\r\n")
	_ = sesh.Exit(1)
	_ = sesh.Close()
}

func WithWebTunnel(handler WebTunnel) ssh.Option {
	return func(serv *ssh.Server) error {
		if serv.ChannelHandlers == nil {
			serv.ChannelHandlers = map[string]ssh.ChannelHandler{
				"session": ssh.DefaultSessionHandler,
			}
		}
		serv.ChannelHandlers["direct-tcpip"] = localForwardHandler(handler)
		return nil
	}
}

type ctxListenerKey struct{}

func getListenerCtx(ctx ssh.Context) (net.Listener, error) {
	listener, ok := ctx.Value(ctxListenerKey{}).(net.Listener)
	if listener == nil || !ok {
		return nil, fmt.Errorf("listener not set on `ssh.Context()` for connection")
	}
	return listener, nil
}
func setListenerCtx(ctx ssh.Context, listener net.Listener) {
	ctx.SetValue(ctxListenerKey{}, listener)
}

func httpServe(handler WebTunnel, ctx ssh.Context) (net.Listener, error) {
	cached, _ := getListenerCtx(ctx)
	if cached != nil {
		return cached, nil
	}

	listener, err := handler.CreateListener(ctx)
	if err != nil {
		return nil, err
	}
	setListenerCtx(ctx, listener)

	go func() {
		httpHandler := handler.GetHttpHandler()
		err := http.Serve(listener, httpHandler(ctx))
		if err != nil {
			log.Println("Unable to serve http content:", err)
		}
	}()

	return listener, nil
}

func localForwardHandler(handler WebTunnel) LocalForwardFn {
	return func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		check := &forwardedTCPPayload{}
		err := gossh.Unmarshal(newChan.ExtraData(), check)
		if err != nil {
			log.Println("Error unmarshaling information:", err)
			return
		}
		log.Printf("%+v", check)

		ch, reqs, err := newChan.Accept()
		if err != nil {
			// TODO: trigger event callback
			return
		}
		go gossh.DiscardRequests(reqs)

		listener, err := httpServe(handler, ctx)
		if err != nil {
			log.Println("Unable to create listener:", err)
			newChan.Reject(gossh.ConnectionFailed, err.Error())
			return
		}
		defer listener.Close()

		go func() {
			downConn, err := handler.CreateConn(ctx)
			if err != nil {
				log.Println("Unable to connect to unix socket:", err)
				newChan.Reject(gossh.ConnectionFailed, err.Error())
				return
			}
			defer downConn.Close()

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				defer ch.Close()
				defer downConn.Close()
				io.Copy(ch, downConn)
			}()
			go func() {
				defer wg.Done()
				defer ch.Close()
				defer downConn.Close()
				io.Copy(downConn, ch)
			}()

			wg.Wait()
		}()

		conn.Wait()
	}
}
