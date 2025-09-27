package router

import (
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-logr/logr"
	kubeauth "github.com/mariadb-operator/mariadb-operator/v25/pkg/kubernetes/auth"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	CompressLevel     int
	RateLimitRequests *int
	RateLimitDuration *time.Duration
	KubernetesAuth    bool
	KubernetesTrusted *kubeauth.Trusted
	BasicAuth         bool
	BasicAuthCreds    map[string]string
}

type Option func(*Options)

func WithCompressLevel(level int) Option {
	return func(o *Options) {
		o.CompressLevel = level
	}
}

func WithRateLimit(requests int, duration time.Duration) Option {
	return func(o *Options) {
		if requests != 0 && duration != 0 {
			o.RateLimitRequests = &requests
			o.RateLimitDuration = &duration
		}
	}
}

func WithKubernetesAuth(auth bool, trusted *kubeauth.Trusted) Option {
	return func(o *Options) {
		o.KubernetesAuth = auth
		o.KubernetesTrusted = trusted
	}
}

func WithBasicAuth(auth bool, user, pass string) Option {
	return func(o *Options) {
		o.BasicAuth = auth
		o.BasicAuthCreds = map[string]string{
			user: pass,
		}
	}
}

type RouteHandler interface {
	SetupRoutes(*chi.Mux)
}

type ProbeHandler interface {
	Liveness(w http.ResponseWriter, r *http.Request)
	Readiness(w http.ResponseWriter, r *http.Request)
}

func NewRouter(apiHandlder RouteHandler, k8sClient ctrlclient.Client, logger logr.Logger, opts ...Option) http.Handler {
	routerOpts := Options{
		CompressLevel:  5,
		KubernetesAuth: false,
		BasicAuth:      false,
	}
	for _, setOpt := range opts {
		setOpt(&routerOpts)
	}
	r := chi.NewRouter()
	r.Use(middleware.Compress(routerOpts.CompressLevel))
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Mount("/api", apiRouter(apiHandlder, k8sClient, logger, &routerOpts))

	return r
}

func NewProbeRouter(handler ProbeHandler, logger logr.Logger, opts ...Option) http.Handler {
	routerOpts := Options{
		CompressLevel: 5,
	}
	for _, setOpt := range opts {
		setOpt(&routerOpts)
	}
	routerOpts.KubernetesAuth = false
	routerOpts.BasicAuth = false

	r := chi.NewRouter()
	r.Use(middleware.Compress(routerOpts.CompressLevel))
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/liveness", handler.Liveness)
	r.Get("/readiness", handler.Readiness)

	return r
}

func apiRouter(handler RouteHandler, k8sClient ctrlclient.Client, logger logr.Logger, opts *Options) http.Handler {
	r := chi.NewRouter()
	if opts.RateLimitRequests != nil && opts.RateLimitDuration != nil {
		r.Use(httprate.LimitAll(*opts.RateLimitRequests, *opts.RateLimitDuration))
	}
	r.Use(middleware.Logger)
	if opts.KubernetesAuth && opts.KubernetesTrusted != nil {
		kauth := kubeauth.NewKubernetesAuth(k8sClient, opts.KubernetesTrusted, logger)
		r.Use(kauth.Handler)
	} else if opts.BasicAuth && opts.BasicAuthCreds != nil {
		r.Use(middleware.BasicAuth("mariadb-operator", opts.BasicAuthCreds))
	}

	handler.SetupRoutes(r)

	return r
}
