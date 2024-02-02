package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/picosh/ptun"
)

type fakeContext struct {
	context.Context
	sync.Locker
}

func (ctx fakeContext) Value(key interface{}) interface{} {
	return nil
}

func (ctx fakeContext) SetValue(key, value interface{}) {}

func (ctx fakeContext) User() string {
	return ""
}

func (ctx fakeContext) SessionID() string {
	return ""
}

func (ctx fakeContext) ClientVersion() string {
	return ""
}

func (ctx fakeContext) ServerVersion() string {
	return ""
}

func (ctx fakeContext) RemoteAddr() net.Addr {
	return nil
}

func (ctx fakeContext) LocalAddr() net.Addr {
	return nil
}

func (ctx fakeContext) Permissions() *ssh.Permissions {
	return nil
}

func (ctx fakeContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func checkAuthenticationKeyRequest(authUrl string, authKey string, addr net.Addr, user string) (bool, error) {
	parsedUrl, err := url.ParseRequestURI(authUrl)
	if err != nil {
		return false, fmt.Errorf("error parsing url %s", err)
	}

	urlS := parsedUrl.String()
	reqBodyMap := map[string]string{
		"auth_key":    string(authKey),
		"remote_addr": addr.String(),
		"user":        user,
	}
	reqBody, err := json.Marshal(reqBodyMap)
	if err != nil {
		return false, fmt.Errorf("error jsonifying request body")
	}
	res, err := http.Post(urlS, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, err
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("Public key rejected by auth service: %s with status %d", urlS, res.StatusCode)
		return false, nil
	}

	return true, nil
}

func AuthHandler() func(ssh.Context, ssh.PublicKey) bool {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		kb := base64.StdEncoding.EncodeToString(key.Marshal())
		if kb == "" {
			return false
		}
		kk := fmt.Sprintf("%s %s", key.Type(), kb)

		success, err := checkAuthenticationKeyRequest(
			"https://auth.pico.sh/key?space=registry",
			kk,
			ctx.RemoteAddr(),
			ctx.User(),
		)
		if err != nil {
			log.Println(err)
		}
		return success
	}
}

func serveMux(ctx ssh.Context) http.Handler {
	router := http.NewServeMux()

	slug := ""

	key := ctx.Value(ssh.ContextKeyPublicKey)
	if key != nil {
		sshKey := key.(ssh.PublicKey)
		h := sha256.New()
		h.Write(sshKey.Marshal())
		slug = hex.EncodeToString(h.Sum(nil))
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "registry:5000",
	})

	oldDirector := proxy.Director

	proxy.Director = func(r *http.Request) {
		log.Printf("%+v", r)
		oldDirector(r)
		fullPath := strings.TrimPrefix(r.URL.Path, "/v2")

		newPath, err := url.JoinPath("/v2", slug, fullPath)
		if err != nil {
			return
		}

		r.URL.Path = newPath
		log.Printf("%+v", r)
	}

	proxy.ModifyResponse = func(r *http.Response) error {
		log.Printf("%+v", r)

		locationHeader := r.Header.Get("location")
		if slug != "" && strings.Contains(locationHeader, fmt.Sprintf("/v2/%s", slug)) {
			r.Header.Set("location", strings.ReplaceAll(locationHeader, fmt.Sprintf("/v2/%s", slug), "/v2"))
		}

		return nil
	}

	router.HandleFunc("/", proxy.ServeHTTP)

	return router
}

// ssh -L 8081:localhost:3000 -p 2222 -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -N localhost
func main() {
	host := os.Getenv("SSH_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("SSH_PORT")
	if port == "" {
		port = "2222"
	}

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", host, port)),
		wish.WithHostKeyPath("ssh_data/term_info_ed25519"),
		wish.WithPublicKeyAuth(AuthHandler()),
		ptun.WithWebTunnel(serveMux),
	)

	if err != nil {
		log.Fatal(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting SSH server on %s:%s", host, port)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		fakeCtx := fakeContext{}

		if err := http.ListenAndServe(":8080", serveMux(fakeCtx)); err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
