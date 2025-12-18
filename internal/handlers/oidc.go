package handlers

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// OIDCConfiguration represents the OpenID Connect discovery document
type OIDCConfiguration struct {
	TokenEndpoint                     string   `json:"token_endpoint"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	JwksURI                           string   `json:"jwks_uri"`
	ResponseModesSupported            []string `json:"response_modes_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	Issuer                            string   `json:"issuer"`
	RequestURIParameterSupported      bool     `json:"request_uri_parameter_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
}

// OIDCConfigurationHandler handles OIDC discovery endpoint
type OIDCConfigurationHandler struct {
	baseURL string
	issuer  string
	logger  *zap.Logger
}

// NewOIDCConfigurationHandler creates a new OIDC configuration handler
func NewOIDCConfigurationHandler(baseURL, issuer string, logger *zap.Logger) *OIDCConfigurationHandler {
	return &OIDCConfigurationHandler{
		baseURL: baseURL,
		issuer:  issuer,
		logger:  logger,
	}
}

// HandleOIDCConfiguration handles GET /.well-known/openid-configuration
func (h *OIDCConfigurationHandler) HandleOIDCConfiguration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := OIDCConfiguration{
		TokenEndpoint:                     h.baseURL + "/oauth2/v1.0/token",
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
		JwksURI:                           h.baseURL + "/discovery/v1.0/keys",
		ResponseModesSupported:            []string{"query", "fragment", "form_post"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ResponseTypesSupported:            []string{"code", "token"},
		ScopesSupported:                   []string{"openid"},
		Issuer:                            h.issuer,
		RequestURIParameterSupported:      false,
		ClaimsSupported: []string{
			"sub",
			"iss",
			"aud",
			"exp",
			"iat",
			"jti",
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		h.logger.Error("Failed to marshal OIDC configuration", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
