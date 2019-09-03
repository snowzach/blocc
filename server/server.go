package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/blendle/zapdriver"
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
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
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

	defaultSymbol string
	defaultCount  int

	distCache    store.DistCache
	cacheTimeout time.Duration

	blockChainStore blocc.BlockChainStore
	txBus           blocc.TxBus
}

// New will setup the server
func New(blockChainStore blocc.BlockChainStore, txBus blocc.TxBus, distCache store.DistCache) (*Server, error) {

	// This router is used for http requests only, setup all of our middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Log Requests - Use appropriate format depending on the encoding
	if config.GetBool("server.log_requests") {
		switch config.GetString("logger.encoding") {
		case "stackdriver":
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start := time.Now()
					var requestID string
					if reqID := r.Context().Value(middleware.RequestIDKey); reqID != nil {
						requestID = reqID.(string)
					}
					ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
					// Parse the request
					next.ServeHTTP(ww, r)
					// Don't log the version endpoint, it's too noisy
					if r.RequestURI == "/version" {
						return
					}
					// If the remote IP is being proxied, use the real IP
					remoteIP := r.Header.Get("X-Forwarded-For")
					if remoteIP == "" {
						remoteIP = r.RemoteAddr
					}
					zap.L().Info("HTTP Request", []zapcore.Field{
						zapdriver.HTTP(&zapdriver.HTTPPayload{
							RequestMethod: r.Method,
							RequestURL:    r.RequestURI,
							RequestSize:   strconv.FormatInt(r.ContentLength, 10),
							Status:        ww.Status(),
							ResponseSize:  strconv.Itoa(ww.BytesWritten()),
							UserAgent:     r.UserAgent(),
							RemoteIP:      remoteIP,
							Referer:       r.Referer(),
							Latency:       fmt.Sprintf("%fs", time.Since(start).Seconds()),
							Protocol:      r.Proto,
						}),
						zap.String("request-id", requestID),
					}...)
				})
			})
		default:
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start := time.Now()
					var requestID string
					if reqID := r.Context().Value(middleware.RequestIDKey); reqID != nil {
						requestID = reqID.(string)
					}
					ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
					next.ServeHTTP(ww, r)

					// Don't log the version endpoint, it's too noisy
					if r.RequestURI == "/version" {
						return
					}

					latency := time.Since(start)

					fields := []zapcore.Field{
						zap.Int("status", ww.Status()),
						zap.Duration("took", latency),
						zap.String("request", r.RequestURI),
						zap.String("method", r.Method),
						zap.String("package", "server.request"),
					}
					if requestID != "" {
						fields = append(fields, zap.String("request-id", requestID))
					}
					// If we have an x-Forwarded-For header, use that for the remote
					if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
						fields = append(fields, zap.String("remote", forwardedFor))
					} else {
						fields = append(fields, zap.String("remote", r.RemoteAddr))
					}
					zap.L().Info("HTTP Request", fields...)
				})
			})
		}
	}

	// CORS Config
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

		defaultSymbol: config.GetString("server.default_symbol"),
		defaultCount:  config.GetInt("server.default_count"),

		distCache:    distCache,
		cacheTimeout: config.GetDuration("server.cache_duration"),

		blockChainStore: blockChainStore,
		txBus:           txBus,
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
	s.router.Get("/version", s.GetVersion())

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

// gwReg will save a gateway registration function for later when the server is started
func (s *Server) gwReg(gwrf gwRegFunc) {
	s.gwRegFuncs = append(s.gwRegFuncs, gwrf)
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
