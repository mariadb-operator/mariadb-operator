package router

import (
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/handler"
	kubeauth "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/auth"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	CompressLevel     int
	RateLimitRequests *int
	RateLimitDuration *time.Duration
	KubernetesAuth    bool
	KubernetesTrusted *kubeauth.Trusted
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

func NewRouter(handler *handler.Handler, k8sClient ctrlclient.Client, logger logr.Logger, opts ...Option) http.Handler {
	routerOpts := Options{
		CompressLevel:     5,
		KubernetesAuth:    false,
		KubernetesTrusted: nil,
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
	r.Mount("/api", apiRouter(handler, k8sClient, logger, &routerOpts))
	r.Get("/liveness", handler.Probe.Liveness)
	r.Get("/readiness", handler.Probe.Readiness)

	return r
}

func apiRouter(h *handler.Handler, k8sClient ctrlclient.Client, logger logr.Logger, opts *Options) http.Handler {
	r := chi.NewRouter()
	if opts.RateLimitRequests != nil && opts.RateLimitDuration != nil {
		r.Use(httprate.LimitAll(*opts.RateLimitRequests, *opts.RateLimitDuration))
	}
	r.Use(middleware.Logger)
	if opts.KubernetesAuth && opts.KubernetesTrusted != nil {
		kauth := kubeauth.NewKubernetesAuth(k8sClient, opts.KubernetesTrusted, logger)
		r.Use(kauth.Handler)
	}

	r.Route("/bootstrap", func(r chi.Router) {
		r.Get("/", h.Bootstrap.IsBootstrapEnabled)
		r.Put("/", h.Bootstrap.Enable)
		r.Delete("/", h.Bootstrap.Disable)
	})
	r.Route("/state", func(r chi.Router) {
		r.Get("/galera", h.State.GetGaleraState)
	})
	r.Route("/recovery", func(r chi.Router) {
		r.Put("/", h.Recovery.Enable)
		r.Post("/", h.Recovery.Start)
		r.Delete("/", h.Recovery.Disable)
	})

	return r
}
