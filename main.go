package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/acsermely/veracy.server/src/config"
	"github.com/acsermely/veracy.server/src/db"
	"github.com/acsermely/veracy.server/src/distributed"
	"github.com/acsermely/veracy.server/src/handlers"
)

func main() {
	conf := config.Parse()

	db, err := db.Create()
	if err != nil {
		log.Fatal("Failed to init DB")
	}
	defer db.Close()

	port := fmt.Sprintf(":%d", conf.Port)

	server := initServer(port)
	initDistributedConnection(&conf)

	log.Printf("Server started at %v\n", port)
	err = server.ListenAndServeTLS("", "")
	// err = server.ListenAndServe()
	if err != nil {
		log.Fatalf("failed to start server: %s", err)
	}
}

func initServer(port string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/img", handlers.WalletMiddleware(handlers.Image))
	mux.HandleFunc("/upload", handlers.WalletMiddleware(handlers.Upload))
	mux.HandleFunc("/loginCheck", handlers.WalletMiddleware(handlers.LoginCheckKey))

	mux.HandleFunc("/registerKey", handlers.Register)
	mux.HandleFunc("/challange", handlers.GetLoginChal)
	mux.HandleFunc("/loginChal", handlers.LoginWhitChal)

	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	server := &http.Server{
		Addr:      port,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	return server
}

func initDistributedConnection(conf *config.AppConfig) *distributed.ContentNode {
	return distributed.Connect(conf)
}
