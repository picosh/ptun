package ptun

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
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

type Handler = func(ctx ssh.Context) http.Handler

func WithWebTunnel(handler Handler) ssh.Option {
	return func(s *ssh.Server) error {
		if s.ChannelHandlers == nil {
			s.ChannelHandlers = map[string]ssh.ChannelHandler{}
		}
		s.ChannelHandlers["direct-tcpip"] = CreateDirectTcpIpHandler(handler)
		return nil
	}
}

func CreateDirectTcpIpHandler(handler Handler) func(*ssh.Server, *gossh.ServerConn, gossh.NewChannel, ssh.Context) {
	return func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		tempFile, err := os.CreateTemp("", "")
		if err != nil {
			log.Println("Unable to create tempfile:", err)
			return
		}

		tempFile.Close()
		os.Remove(tempFile.Name())

		connListener, err := net.Listen("unix", tempFile.Name())
		if err != nil {
			log.Println("Unable to listen to unix socket:", err)
			return
		}

		go func() {
			if err := http.Serve(connListener, handler(ctx)); err != nil {
				log.Println("Unable to serve http content:", err)
			}
		}()

		defer connListener.Close()
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
			downConn, err := net.Dial("unix", tempFile.Name())
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
