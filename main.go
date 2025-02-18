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

	// os.Setenv("ADMIN_KEY", `{"kty":"RSA","n":"nLzWkkzEyymQsh1yRNYP3iJm1slMukAK_kUVltiW2WbkJ_1x-SndqLf4jbVDB_QSbIwHOpMf1YkfLUqQw8RoHP-ipdjvE1-7fkpP5ieFN-tQEO04vDc2ym8SID7EOIjbyr7pn-Bkk0Cw9ztBA0EY4xPYeI-6NAvWcrcAMhR9GulVSXPrWGoiRg6d9exGjFAySYHetxRu978zMVTyXmmmLeymD6opi8xGBvoMyVWJzwsyj9nYeB19bXduJPUX2AG6DEXLnQ80Y0UiYTWJ27kwHfEBj4xvcoTOzYmruUT7TObSeqgkaothTa8IBVxejCka4QtUHlnD78PahL6rdSCuG8e_klCHBfpLuJhx0dR6RqktJUMooDbZzCpAetQWjI9LhrApS-G1APF74i6cBDSFbJ2FVoiPz2QalhI-aJvbj4CdPY3odqvsSjp5DnjVbrgQNoopqXouYHaTT19NLXm0XLlNwWNlvIlWBaZfyhi6q7hsOUIFYQHYy0fXbCHFPZLDfDsokIlNUAms5OwG9iyJRFdYM5l_MVwO2kdTpkOoyHLssiWzSOCcm6GJ28FMo_CYIKJhzF8iZffJw40KeUTw3-Egmc23zrZm5fmAH190_hKecplPqeaIuFpTv6xzA3Xgz3ffUb_-rFLYC8JU7Li_uAEhXksWOuboWo0WQ8otyZ0","e":"AQAB","alg":"RSA-OAEP-256","ext":true,"key_ops":["encrypt"]}`)
	// os.Setenv("SECRET", `secret`)

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
	mux.HandleFunc("/loginCheck", handlers.WalletMiddleware(handlers.LoginCheckKey))
	mux.HandleFunc("/feedback", handlers.WalletMiddleware(handlers.AddFeedback))

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
