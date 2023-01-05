package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

    "os"
    "log"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/middleware"
	"github.com/docker/docker/api/server/router"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

// versionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const versionMatcher = "/v{version:[0-9.]+}"

// Config provides the configuration for the API server
type Config struct {
	Logging     bool
	EnableCors  bool
	CorsHeaders string
	Version     string
	SocketGroup string
	TLSConfig   *tls.Config
}

// Server contains instance details for the server
type Server struct {
	cfg           *Config
	servers       []*HTTPServer
	routers       []router.Router
	routerSwapper *routerSwapper
	middlewares   []middleware.Middleware
}

// New returns a new instance of the server based on the specified configuration.
// It allocates resources which will be needed for ServeAPI(ports, unix-sockets).
func New(cfg *Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

// UseMiddleware appends a new middleware to the request chain.
// This needs to be called before the API routes are configured.
func (s *Server) UseMiddleware(m middleware.Middleware) {
	s.middlewares = append(s.middlewares, m)
}

// Accept sets a listener the server accepts connections into.
func (s *Server) Accept(addr string, listeners ...net.Listener) {
    fmt.Println("api/server/server  Accept()")
    //    logPrintAPIServer("Accept()")
	for _, listener := range listeners {
		httpServer := &HTTPServer{
			srv: &http.Server{
				Addr: addr,
			},
			l: listener,
		}
		s.servers = append(s.servers, httpServer)
	}
}

// Close closes servers and thus stop receiving requests
func (s *Server) Close() {
	for _, srv := range s.servers {
		if err := srv.Close(); err != nil {
			logrus.Error(err)
		}
	}
}

// serveAPI loops through all initialized servers and spawns goroutine
// with Serve method for each. It sets createMux() as Handler also.
func (s *Server) serveAPI() error {
    fmt.Println("api/server/server  serveAPI()")
	var chErrors = make(chan error, len(s.servers))
	for _, srv := range s.servers {
		srv.srv.Handler = s.routerSwapper
        //logPrintAPIServer("API listen Addr : " + srv.l.Addr())
        logPrintAPIServer("serveAPI()")
		go func(srv *HTTPServer) {
			var err error
			logrus.Infof("API listen on %s", srv.l.Addr())
            fmt.Println("api/server/server  serveAPI() go func() API listen on : ", srv.l.Addr())
            //logPrintAPIServer("API listen Addr go : " + srv.l.Addr())
            logPrintAPIServer("serveAPI go()")
			if err = srv.Serve(); err != nil && strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			}
			chErrors <- err
		}(srv)
	}

	for i := 0; i < len(s.servers); i++ {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

// HTTPServer contains an instance of http server and the listener.
// srv *http.Server, contains configuration to create an http server and a mux router with all api end points.
// l   net.Listener, is a TCP or Socket listener that dispatches incoming request to the router.
type HTTPServer struct {
	srv *http.Server
	l   net.Listener
}

// Serve starts listening for inbound requests.
func (s *HTTPServer) Serve() error {
    logPrintAPIServer("API Server Serve")
	return s.srv.Serve(s.l)
}

// Close closes the HTTPServer from listening for the inbound requests.
func (s *HTTPServer) Close() error {
	return s.l.Close()
}

func (s *Server) makeHTTPHandler(handler httputils.APIFunc) http.HandlerFunc {
    fmt.Println("api/server/server.go  makeHTTPHandler()")
	return func(w http.ResponseWriter, r *http.Request) {
		// Define the context that we'll pass around to share info
		// like the docker-request-id.
		//
		// The 'context' will be used for global data that should
		// apply to all requests. Data that is specific to the
		// immediate function being called should still be passed
		// as 'args' on the function call.
		ctx := context.WithValue(context.Background(), httputils.UAStringKey, r.Header.Get("User-Agent"))
		
        fmt.Println("api/server/server.go  handlerWithGlobalMiddlewares()")
        handlerFunc := s.handlerWithGlobalMiddlewares(handler)
        fmt.Println("api/server/server.go  makeHTTPHandler() handlerFunc :", handlerFunc)
		
        vars := mux.Vars(r)
		if vars == nil {
			vars = make(map[string]string)
		}

        fmt.Println("api/server/server.go makeHTTPHandler() method : ", r.Method)
        logPrintAPIServer(r.Method)
        fmt.Println("api/server/server.go makeHTTPHandler() urlPath : ", r.URL.Path)
        logPrintAPIServer(r.URL.Path)
		if err := handlerFunc(ctx, w, r, vars); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %v", r.Method, r.URL.Path, err)
			httputils.MakeErrorHandler(err)(w, r)
		}

	}
}

// InitRouter initializes the list of routers for the server.
// This method also enables the Go profiler if enableProfiler is true.
func (s *Server) InitRouter(enableProfiler bool, routers ...router.Router) {
    fmt.Println("cmd/server/server.go   InitRouter()")
    //logPrintAPIServer("InitRouter()")
	s.routers = append(s.routers, routers...)

	m := s.createMux()
	if enableProfiler {
		profilerSetup(m)
	}
	s.routerSwapper = &routerSwapper{
		router: m,
	}
}

// createMux initializes the main router the server uses.
func (s *Server) createMux() *mux.Router {
    fmt.Println("api/server/server.go  createMux()")
	m := mux.NewRouter()

	logrus.Debug("Registering routers")
	for _, apiRouter := range s.routers {
		for _, r := range apiRouter.Routes() {
			f := s.makeHTTPHandler(r.Handler())

			logrus.Debugf("Registering %s, %s", r.Method(), r.Path())
			m.Path(versionMatcher + r.Path()).Methods(r.Method()).Handler(f)
			m.Path(r.Path()).Methods(r.Method()).Handler(f)
		}
	}

	err := errors.NewRequestNotFoundError(fmt.Errorf("page not found"))
	notFoundHandler := httputils.MakeErrorHandler(err)
	m.HandleFunc(versionMatcher+"/{path:.*}", notFoundHandler)
	m.NotFoundHandler = notFoundHandler

	return m
}

// Wait blocks the server goroutine until it exits.
// It sends an error message if there is any error during
// the API execution.
func (s *Server) Wait(waitChan chan error) {
	if err := s.serveAPI(); err != nil {
		logrus.Errorf("ServeAPI error: %v", err)
		waitChan <- err
		return
	}
	waitChan <- nil
}

// DisableProfiler reloads the server mux without adding the profiler routes.
func (s *Server) DisableProfiler() {
	s.routerSwapper.Swap(s.createMux())
}

// EnableProfiler reloads the server mux adding the profiler routes.
func (s *Server) EnableProfiler() {
	m := s.createMux()
	profilerSetup(m)
	s.routerSwapper.Swap(m)
}


func logPrintAPIServer(errStr string) {
    logFile, logError := os.Open("/home/vagrant/logAPIServer.md")
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/logAPIServer.md")
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
