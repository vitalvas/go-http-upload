package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"
)

func logger(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Access [%s] %s", r.Method, r.URL.Path)

		handler(w, r)

		log.Printf("Completed %q\n", time.Since(start))
	}
}

func HideDir(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func BasicAuth(handler http.HandlerFunc, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authError := func() {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Zork\"")
			http.Error(w, "authorization failed", http.StatusUnauthorized)
		}

		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			authError()
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			authError()
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || !(pair[0] == username && pair[1] == password) {
			authError()
			return
		}

		handler(w, r)
	}
}
