package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/gogo/gateway"
	"github.com/gogo/protobuf/jsonpb"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/snowzach/certtools"
	"github.com/snowzach/certtools/autocert"
	config "github.com/spf13/viper"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"git.coinninja.net/backend/blocc/server/versionrpc/versionrpcserver"
)

// When starting to listen, we will reigster gateway functions
type gwRegFunc func(ctx context.Context, mux *gwruntime.ServeMux, endpoint string, opts []grpc.DialOption) error

// Server is the GRPC server
type Server struct {
	logger     *zap.SugaredLogger
	router     chi.Router
	server     *http.Server
	grpcServer *grpc.Server
	gwRegFuncs []gwRegFunc
}

// New will setup the server
func New() (*Server, error) {

	// This router is used for http requests only, setup all of our middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodHead, http.MethodOptions, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler)

	// GRPC Interceptors
	streamInterceptors := []grpc.StreamServerInterceptor{}
	unaryInterceptors := []grpc.UnaryServerInterceptor{}

	// Log Requests - Use appropriate format depending on the encoding
	if config.GetBool("server.log_requests") {
		switch config.GetString("logger.encoding") {
		case "stackdriver":
			unaryInterceptors = append(unaryInterceptors, loggerGRPCUnaryStackdriver(config.GetBool("server.log_requests_body"), config.GetStringSlice("server.log_disabled_grpc")))
			streamInterceptors = append(streamInterceptors, loggerGRPCStreamStackdriver(config.GetStringSlice("server.log_disabled_grpc_stream")))
			r.Use(loggerHTTPMiddlewareStackdriver(config.GetBool("server.log_requests_body"), config.GetStringSlice("server.log_disabled_http")))
		default:
			unaryInterceptors = append(unaryInterceptors, loggerGRPCUnaryDefault(config.GetBool("server.log_requests_body"), config.GetStringSlice("server.log_disabled_grpc")))
			streamInterceptors = append(streamInterceptors, loggerGRPCStreamDefault(config.GetStringSlice("server.log_disabled_grpc_stream")))
			r.Use(loggerHTTPMiddlewareDefault(config.GetBool("server.log_requests_body"), config.GetStringSlice("server.log_disabled_http")))
		}
	}

	// GRPC Server Options
	serverOptions := []grpc.ServerOption{
		grpc_middleware.WithStreamServerChain(streamInterceptors...),
		grpc_middleware.WithUnaryServerChain(unaryInterceptors...),
	}

	// Create gRPC Server
	g := grpc.NewServer(serverOptions...)
	// Register reflection service on gRPC server (so people know what we have)
	reflection.Register(g)

	s := &Server{
		logger:     zap.S().With("package", "server"),
		router:     r,
		grpcServer: g,
		gwRegFuncs: make([]gwRegFunc, 0),
	}
	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
				g.ServeHTTP(w, r)
			} else {
				s.router.ServeHTTP(w, r)
			}
		}),
		ErrorLog: log.New(&serverLogger{logger: zap.S().With("package", "server.wsproxy")}, "", 0),
	}

	s.SetupRoutes()

	return s, nil

}

// NewHealthServer returns OK to pretty every response - just used as a health check endpoint
func NewHealthServer() (*Server, error) {

	// Create the server object
	s := &Server{
		logger: zap.S().With("package", "server"),
		router: chi.NewRouter(),
	}
	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.router.ServeHTTP(w, r)
		}),
	}

	// Enable the version endpoint
	s.router.Get("/version", versionrpcserver.GetVersion())

	// Every Route Returns OK
	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		render.PlainText(w, r, "OK")
	})

	return s, nil

}

// ListenAndServe will listen for requests
func (s *Server) ListenAndServe() error {

	s.server.Addr = net.JoinHostPort(config.GetString("server.host"), config.GetString("server.port"))

	// Listen
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("Could not listen on %s: %v", s.server.Addr, err)
	}

	grpcGatewayDialOptions := []grpc.DialOption{}

	// Enable TLS?
	if config.GetBool("server.tls") {
		var cert tls.Certificate
		if config.GetBool("server.devcert") {
			s.logger.Warn("WARNING: This server is using an insecure development tls certificate. This is for development only!!!")
			cert, err = autocert.New(autocert.InsecureStringReader("localhost"))
			if err != nil {
				return fmt.Errorf("Could not autocert generate server certificate: %v", err)
			}
		} else {
			// Load keys from file
			cert, err = tls.LoadX509KeyPair(config.GetString("server.certfile"), config.GetString("server.keyfile"))
			if err != nil {
				return fmt.Errorf("Could not load server certificate: %v", err)
			}
		}

		// Enabed Certs - TODO Add/Get a cert
		s.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   certtools.SecureTLSMinVersion(),
			CipherSuites: certtools.SecureTLSCipherSuites(),
			NextProtos:   []string{"h2"},
		}
		// Wrap the listener in a TLS Listener
		listener = tls.NewListener(listener, s.server.TLSConfig)

		// Fetch the CommonName from the certificate and generate a cert pool for the grpc gateway to use
		// This essentially figures out whatever certificate we happen to be using and makes it valid for the call between the GRPC gateway and the GRPC endpoint
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("Could not parse x509 public cert from tls certificate: %v", err)
		}
		clientCertPool := x509.NewCertPool()
		clientCertPool.AddCert(x509Cert)
		grpcCreds := credentials.NewClientTLSFromCert(clientCertPool, x509Cert.Subject.CommonName)
		grpcGatewayDialOptions = append(grpcGatewayDialOptions, grpc.WithTransportCredentials(grpcCreds))

	} else {
		// This h2c helper allows using insecure requests to http2/grpc
		s.server.Handler = h2c.NewHandler(s.server.Handler, &http2.Server{})
		grpcGatewayDialOptions = append(grpcGatewayDialOptions, grpc.WithInsecure())
	}

	// If we haven't specified the grpc server, don't initialize all the grpc endpoints
	if s.grpcServer != nil {

		// Setup the GRPC gateway - use gogoproto's marshaler
		grpcGatewayJSONpbMarshaler := gateway.JSONPb(jsonpb.Marshaler{
			EnumsAsInts:  config.GetBool("server.rest.enums_as_ints"),
			EmitDefaults: config.GetBool("server.rest.emit_defaults"),
			OrigName:     config.GetBool("server.rest.orig_names"),
		})
		grpcGatewayMux := gwruntime.NewServeMux(gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &grpcGatewayJSONpbMarshaler))

		// Register all the GRPC gateway functions
		for _, gwrf := range s.gwRegFuncs {
			err = gwrf(context.Background(), grpcGatewayMux, listener.Addr().String(), grpcGatewayDialOptions)
			if err != nil {
				return fmt.Errorf("Could not register HTTP/gRPC gateway: %s", err)
			}
		}

		// Wrap the grpcGateway in the websocket proxy helper and our server logger
		wsGrpcMux := wsproxy.WebsocketProxy(grpcGatewayMux, wsproxy.WithLogger(&serverLogger{logger: s.logger}))

		// If the main router did not find and endpoint, pass it to the grpcGateway
		s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
			wsGrpcMux.ServeHTTP(w, r)
		})

	}

	go func() {
		if err = s.server.Serve(listener); err != nil {
			s.logger.Fatalw("API Listen error", "error", err, "address", s.server.Addr)
		}
	}()
	s.logger.Infow("API Listening", "address", s.server.Addr, "tls", config.GetBool("server.tls"))

	// Enable profiler
	if config.GetBool("server.profiler_enabled") && config.GetString("server.profiler_path") != "" {
		zap.S().Debugw("Profiler enabled on API", "path", config.GetString("server.profiler_path"))
		s.router.Mount(config.GetString("server.profiler_path"), middleware.Profiler())
	}

	return nil

}

// GwReg will save a gateway registration function for later when the server is started
func (s *Server) GwReg(gwrf gwRegFunc) {
	s.gwRegFuncs = append(s.gwRegFuncs, gwrf)
}

// GRPCServer will return the grpc server to allow functions to register themselves
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Router will return the router to allow functions to register themselves
func (s *Server) Router() chi.Router {
	return s.router
}

// serverLogger is a multi-purpose logger used for server error messsages and websocket interceptor messages
type serverLogger struct {
	logger *zap.SugaredLogger
}

// Write is called by the server when there it an error
func (sl *serverLogger) Write(b []byte) (int, error) {
	sl.logger.Error(string(b))
	return len(b), nil
}

// Warnln is called by the wsproxy for warning messages
func (sl *serverLogger) Warnln(data ...interface{}) {
	sl.logger.Warn(data...)
}

// Debugln is called by the wsproxy for debug messages
func (sl *serverLogger) Debugln(data ...interface{}) {
	sl.logger.Debug(data...)
}

// RenderOrErrInternal will render whatever you pass it (assuming it has Renderer) or prints an internal error
func RenderOrErrInternal(w http.ResponseWriter, r *http.Request, d render.Renderer) {
	if err := render.Render(w, r, d); err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}
}
