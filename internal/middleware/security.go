package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"

	"github.com/a-h/templ"
)

const (
	headerXContentTypeOptions = "X-Content-Type-Options"
	headerReferrerPolicy      = "Referrer-Policy"
	headerPermissionsPolicy   = "Permissions-Policy"
	headerCOOP                = "Cross-Origin-Opener-Policy"
	headerCOEP                = "Cross-Origin-Embedder-Policy"
	headerCORP                = "Cross-Origin-Resource-Policy"
	headerCSP                 = "Content-Security-Policy"
	headerACAllowOrigin       = "Access-Control-Allow-Origin"
	headerACAllowMethods      = "Access-Control-Allow-Methods"
	headerACMaxAge            = "Access-Control-Max-Age"
	headerVary                = "Vary"
)

var frontendStaticHeaders = map[string]string{
	headerXContentTypeOptions: "nosniff",
	headerReferrerPolicy:      "no-referrer",
	headerPermissionsPolicy: "geolocation=(), camera=(), microphone=(), payment=(), " +
		"usb=(), magnetometer=(), gyroscope=(), accelerometer=(), " +
		"autoplay=(), picture-in-picture=(), display-capture=(), " +
		"encrypted-media=(), web-share=(), interest-cohort=()",
	headerCOOP: "same-origin",
	headerCOEP: "require-corp",
	headerCORP: "same-origin",
}

var apiHeaders = map[string]string{
	headerXContentTypeOptions: "nosniff",
	headerCORP:                "cross-origin",
	headerACAllowOrigin:       "*",
	headerACAllowMethods:      "GET, OPTIONS",
	headerACMaxAge:            "86400",
	headerVary:                "Origin",
}

func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate nonce: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func buildCSP(nonce string) string {
	return fmt.Sprintf(
		"default-src 'none'; "+
			"script-src 'self' 'nonce-%s'; "+
			"script-src-elem 'self' 'nonce-%s'; "+
			"style-src 'self' 'nonce-%s'; "+
			"style-src-attr 'unsafe-inline'; "+
			"img-src 'self' data: blob:; "+
			"font-src 'self'; "+
			"media-src 'self' blob: data:; "+
			"connect-src 'self' blob:; "+
			"frame-ancestors 'none'; "+
			"base-uri 'self'; "+
			"form-action 'self'",
		nonce, nonce, nonce,
	)
}

func FrontendSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range frontendStaticHeaders {
			w.Header().Set(k, v)
		}

		nonce := generateNonce()
		w.Header().Set(headerCSP, buildCSP(nonce))
		ctx := templ.WithNonce(r.Context(), nonce)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ApiSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range apiHeaders {
			w.Header().Set(k, v)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// PathSanitize applies path.Clean to the request URL path before routing.
// This normalizes double slashes, removes dot segments, and prevents path traversal.
func PathSanitize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		if cleaned != r.URL.Path {
			r.URL.Path = cleaned
		}
		next.ServeHTTP(w, r)
	})
}
