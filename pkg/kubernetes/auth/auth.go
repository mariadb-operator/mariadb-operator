package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	agenterrors "github.com/mariadb-operator/agent/pkg/errors"
	"github.com/mariadb-operator/agent/pkg/responsewriter"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Trusted struct {
	ServiceAccountName      string
	ServiceAccountNamespace string
}

func (t *Trusted) String() string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", t.ServiceAccountNamespace, t.ServiceAccountName)
}

type KubernetesAuth struct {
	clientset      *kubernetes.Clientset
	trusted        *Trusted
	responseWriter *responsewriter.ResponseWriter
	logger         logr.Logger
}

func NewKubernetesAuth(clientset *kubernetes.Clientset, trusted *Trusted, logger logr.Logger) *KubernetesAuth {
	return &KubernetesAuth{
		clientset:      clientset,
		trusted:        trusted,
		responseWriter: responsewriter.NewResponseWriter(&logger),
		logger:         logger,
	}
}

func (a *KubernetesAuth) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token, err := authToken(r)
		if err != nil {
			a.logger.V(1).Info("Error getting Authorization header", "err", err)
			a.responseWriter.Write(w, agenterrors.NewAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		tokenReview := &authv1.TokenReview{
			Spec: authv1.TokenReviewSpec{
				Token: token,
			},
		}
		tokenReviewRes, err := a.clientset.AuthenticationV1().TokenReviews().Create(r.Context(), tokenReview, metav1.CreateOptions{})
		if err != nil {
			a.logger.V(1).Info("Error verifying token in TokenReview API", "err", err)
			a.responseWriter.Write(w, agenterrors.NewAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if !tokenReviewRes.Status.Authenticated {
			a.logger.V(1).Info("TokenReview not valid")
			a.responseWriter.Write(w, agenterrors.NewAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if tokenReviewRes.Status.User.Username == "" {
			a.logger.V(1).Info("Username not found")
			a.responseWriter.Write(w, agenterrors.NewAPIError("unauthorized"), http.StatusUnauthorized)
			return
		}
		if a.trusted.String() != tokenReviewRes.Status.User.Username {
			a.logger.V(1).Info("Username not allowed", "username", tokenReviewRes.Status.User.Username)
			a.responseWriter.Write(w, agenterrors.NewAPIError("forbidden"), http.StatusForbidden)
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
