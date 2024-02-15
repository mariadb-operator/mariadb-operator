package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	authv1 "k8s.io/api/authentication/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Trusted struct {
	ServiceAccountName      string
	ServiceAccountNamespace string
}

func (t *Trusted) String() string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", t.ServiceAccountNamespace, t.ServiceAccountName)
}

type KubernetesAuth struct {
	k8sClient      ctrlclient.Client
	trusted        *Trusted
	responseWriter *mdbhttp.ResponseWriter
	logger         logr.Logger
}

func NewKubernetesAuth(k8sClient ctrlclient.Client, trusted *Trusted, logger logr.Logger) *KubernetesAuth {
	return &KubernetesAuth{
		k8sClient:      k8sClient,
		trusted:        trusted,
		responseWriter: mdbhttp.NewResponseWriter(&logger),
		logger:         logger,
	}
}

func (a *KubernetesAuth) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token, err := authToken(r)
		if err != nil {
			a.logger.V(1).Info("Error getting Authorization header", "err", err)
			a.responseWriter.Write(w, newAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		tokenReview := &authv1.TokenReview{
			Spec: authv1.TokenReviewSpec{
				Token: token,
			},
		}
		if err := a.k8sClient.Create(r.Context(), tokenReview); err != nil {
			a.logger.V(1).Info("Error verifying token in TokenReview API", "err", err)
			a.responseWriter.Write(w, newAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if !tokenReview.Status.Authenticated {
			a.logger.V(1).Info("TokenReview not valid")
			a.responseWriter.Write(w, newAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if tokenReview.Status.User.Username == "" {
			a.logger.V(1).Info("Username not found")
			a.responseWriter.Write(w, newAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if a.trusted.String() != tokenReview.Status.User.Username {
			a.logger.V(1).Info("Username not allowed", "username", tokenReview.Status.User.Username)
			a.responseWriter.Write(w, newAPIError("forbidden"), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func authToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", errors.New("Authorization header not found")
	}
	parts := strings.Split(auth, "Bearer ")
	if len(parts) != 2 {
		return "", errors.New("invalid Authorization header")
	}
	return parts[1], nil
}

type apiError struct {
	Message string `json:"message"`
}

func (e *apiError) Error() string {
	return e.Message
}

func newAPIError(message string) error {
	return &apiError{
		Message: message,
	}
}
