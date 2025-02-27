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
	"github.com/joho/godotenv"
)

func main() {
	conf := config.Parse()
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}

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

	mux.HandleFunc("/upload", handlers.WalletMiddleware(handlers.Upload))
	mux.HandleFunc("/getInfo", handlers.WalletMiddleware(handlers.GetInfo))
	mux.HandleFunc("/feedback", handlers.WalletMiddleware(handlers.AddFeedback))
	mux.HandleFunc("/messages", handlers.WalletMiddleware(handlers.GetMessages))
	mux.HandleFunc("/sendMessage", handlers.WalletMiddleware(handlers.SendMessage))

	mux.HandleFunc("/img", handlers.Image)
	mux.HandleFunc("/registerKey", handlers.Register)
	mux.HandleFunc("/challange", handlers.GetLoginChal)
	mux.HandleFunc("/loginChal", handlers.LoginWhitChal)

	mux.HandleFunc("/adminAllImages", handlers.AdminMiddleware(handlers.GetAllImages))
	mux.HandleFunc("/adminSetImageActivity", handlers.AdminMiddleware(handlers.SetImageActivity))

	mux.HandleFunc("/adminChal", handlers.GetAdminChal)
	mux.HandleFunc("/adminLogin", handlers.LoginAdminChal)

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
