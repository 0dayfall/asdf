package server

import (
	"asdf/internal/db"
	"asdf/internal/rest"
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/gorilla/sessions"
)

const WELL_KNOWN_WEBFINGER = "/.well-known/webfinger"

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func Start(addr string, sessionKey string) {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	db := db.NewData()
	loadDataErr := db.LoadData(path.Join("data", "data.json"))
	if loadDataErr != nil {
		log.Fatalf("Error loading data: %v", loadDataErr)
	}

	store := sessions.NewCookieStore([]byte(sessionKey))

	webFingerHandler := &rest.WebFingerHandler{Data: db}
	http.Handle(WELL_KNOWN_WEBFINGER, webFingerHandler)

	rest.LoadTemplates()
	http.HandleFunc("/", webFingerHandler.HTMLHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
		TLSConfig:    &tls.Config{},
		BaseContext:  func(listener net.Listener) context.Context { return ctx },
	}

	go func() {
		httpServerErr := server.ListenAndServe()
		if httpServerErr == http.ErrServerClosed {
			log.Print(httpServerErr)
		} else {
			log.Fatalf("HTTPS server error: %v", httpServerErr)
		}
	}()

	<-stopChan
	log.Println("Shutting down server gracefully..")
	db.SaveData(path.Join("data", "data.json"))
	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		log.Println("Error shutting down: ", shutdownErr)
	} else {
		log.Println("Server shutdown completed")
	}
}
