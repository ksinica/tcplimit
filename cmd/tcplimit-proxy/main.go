package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ksinica/tcplimit"
	"golang.org/x/time/rate"
)

var (
	proxyPort = flag.Int("port", 8080, "Proxy listening port")
)

// Please don't judge the code here, as it's just a simple vanilla
// HTTP proxy used for live demonstration purposes.
//
// Some boundary conditions are not checked here, along with some errors.

const (
	dialTimeout   = 3 * time.Second
	dialKeepAlive = 30 * time.Second
)

func printError(a ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"Error"}, a...)...)
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "not allowed", http.StatusMethodNotAllowed)
		return
	}

	dest, err := (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: dialKeepAlive,
		DualStack: true,
	}).DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		printError(r.RemoteAddr, "Could not dial host:", err)
		return
	}

	fmt.Println(r.RemoteAddr, "Connected to", r.Host)
	defer fmt.Println(r.RemoteAddr, "Disconnected from", r.Host)

	// We need to send status before hijacking conenction.
	w.WriteHeader(http.StatusOK)

	src, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		dest.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		printError(r.RemoteAddr, "Could not hijack connection:", err)
		return
	}

	go copyConn(dest, src)
	copyConn(src, dest)
}

func copyConn(dst io.WriteCloser, src io.ReadCloser) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}

func handlePutLimit(limiter *tcplimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "not allowed", http.StatusMethodNotAllowed)
			return
		}

		var b bytes.Buffer
		if _, err := io.Copy(&b, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			printError(r.RemoteAddr, "Could not read request:", err)
			return
		}

		limit, err := strconv.ParseFloat(b.String(), 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if limit < 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}

		switch r.URL.Path {
		case "/limits/global":
			limiter.SetGlobalLimit(rate.Limit(limit))
			fmt.Println(r.RemoteAddr, "Global limit set to", limit)
		case "/limits/local":
			limiter.SetLocalLimit(rate.Limit(limit))
			fmt.Println(r.RemoteAddr, "Local limit set to", limit)
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func proxyHandler(limiter *tcplimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Why not use http.ServeMux? When wget connects to our proxy,
		// it uses an empty request path, even if we askeed otherwise...
		if strings.HasPrefix(r.URL.Path, "/limits/") {
			handlePutLimit(limiter).ServeHTTP(w, r)
			return
		}

		handleConnect(w, r)
	}
}

type limitedListener struct {
	net.Listener
	limiter *tcplimit.Limiter
}

func (l *limitedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return l.limiter.LimitConn(conn), nil
}

func wrapListener(listener net.Listener, limiter *tcplimit.Limiter) net.Listener {
	return &limitedListener{
		Listener: listener,
		limiter:  limiter,
	}
}

func run() int {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *proxyPort))
	if err != nil {
		printError("Could not start server:", err)
		return 1
	}

	limiter := tcplimit.NewLimiter()

	fmt.Println("Listening on", *proxyPort)

	err = http.Serve(
		wrapListener(lis, limiter),
		http.HandlerFunc(proxyHandler(limiter)),
	)
	if err != nil {
		if err != http.ErrServerClosed {
			printError("Server abnormally terminated:", err)
			return 1
		}
	}
	return 0
}

func main() {
	flag.Parse()

	os.Exit(run())
}
