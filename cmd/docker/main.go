package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/picosh/pico/db"
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

type ctxUserKey struct{}

func getUserCtx(ctx ssh.Context) (*db.User, error) {
	user, ok := ctx.Value(ctxUserKey{}).(*db.User)
	if user == nil || !ok {
		return user, fmt.Errorf("user not set on `ssh.Context()` for connection")
	}
	return user, nil
}
func setUserCtx(ctx ssh.Context, user *db.User) {
	ctx.SetValue(ctxUserKey{}, user)
}

func checkAuthenticationKeyRequest(authUrl, authToken, authKey, username string, addr net.Addr) (*db.User, error) {
	var user *db.User
	parsedUrl, err := url.ParseRequestURI(authUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing url %s", err)
	}

	urlS := parsedUrl.String()
	reqBodyMap := map[string]string{
		"auth_key":    string(authKey),
		"remote_addr": addr.String(),
		"user":        username,
	}
	reqBody, err := json.Marshal(reqBodyMap)
	if err != nil {
		return nil, fmt.Errorf("error jsonifying request body")
	}
	req, err := http.NewRequest("POST", urlS, bytes.NewBuffer(reqBody))
	req.Header.Add("Authorization", authToken)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error auth service: %s with status %d: %s", urlS, res.StatusCode, err.Error())
		return nil, nil
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Printf("Public key rejected by auth service: %s with status %d", urlS, res.StatusCode)
		return nil, nil
	}

	err = json.NewDecoder(res.Body).Decode(&user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func AuthHandler(authToken string) func(ssh.Context, ssh.PublicKey) bool {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		kb := base64.StdEncoding.EncodeToString(key.Marshal())
		if kb == "" {
			return false
		}
		kk := fmt.Sprintf("%s %s", key.Type(), kb)

		user, err := checkAuthenticationKeyRequest(
			"https://auth.pico.sh/key?space=registry",
			authToken,
			kk,
			ctx.User(),
			ctx.RemoteAddr(),
		)
		if err != nil {
			log.Println(err)
			return false
		}

		if user != nil {
			setUserCtx(ctx, user)
			return true
		}

		return false
	}
}

type ErrorHandler struct {
	Err error
}

func (e *ErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(e.Err.Error())
	http.Error(w, e.Err.Error(), http.StatusInternalServerError)
}

func serveMux(ctx ssh.Context) http.Handler {
	router := http.NewServeMux()

	slug := ""
	user, err := getUserCtx(ctx)
	if err == nil && user != nil {
		slug = user.Name
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
	authToken := os.Getenv("AUTH_TOKEN")

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", host, port)),
		wish.WithHostKeyPath("ssh_data/term_info_ed25519"),
		wish.WithPublicKeyAuth(AuthHandler(authToken)),
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
