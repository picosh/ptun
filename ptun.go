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
type HttpHandlerFn = func(sesh ssh.Session) http.Handler

type WebTunnel interface {
	GetHttpHandler() HttpHandlerFn
	CreateListener(sesh ssh.Session) (net.Listener, error)
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
			serv.ChannelHandlers = map[string]ssh.ChannelHandler{}
		}
		serv.ChannelHandlers["direct-tcpip"] = localForwardHandler(handler)
		serv.Handler = sshHandler(handler)
		return nil
	}
}

func sshHandler(handler WebTunnel) ssh.Handler {
	return func(sesh ssh.Session) {
		listener, err := handler.CreateListener(sesh)
		if err != nil {
			ErrorHandler(sesh, err)
			return
		}

		defer func() {
			if r := recover(); r != nil {
				_, _ = sesh.Stderr().Write([]byte("error running middleware\r\n"))
			}
			listener.Close()
		}()

		go func() {
			httpHandler := handler.GetHttpHandler()
			err := http.Serve(listener, httpHandler(sesh))
			if err != nil {
				log.Println("Unable to serve http content:", err)
			}
		}()
	}
}

func localForwardHandler(handler WebTunnel) LocalForwardFn {
	return func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		ch, reqs, err := newChan.Accept()
		if err != nil {
			// TODO: trigger event callback
			return
		}
		check := &forwardedTCPPayload{}
		err = gossh.Unmarshal(newChan.ExtraData(), check)
		if err != nil {
			log.Println("Error unmarshaling information:", err)
			return
		}

		log.Printf("%+v", check)

		go gossh.DiscardRequests(reqs)

		go func() {
			downConn, err := handler.CreateConn(ctx)
			if err != nil {
				log.Println("Unable to connect to unix socket:", err)
				return
			}

			defer downConn.Close()

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				io.Copy(ch, downConn)
				ch.CloseWrite()
			}()
			go func() {
				defer wg.Done()
				io.Copy(downConn, ch)
				downConn.Close()
			}()

			wg.Wait()
		}()

		conn.Wait()
	}
}
