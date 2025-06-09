package customcors

import (
	"fmt"
	"mediaserver/utils/dotenv"

	"github.com/rs/cors"
)

func SetupCors() *cors.Cors {
	frontendHost := dotenv.GetDotEnv("FE_URL")
	frontendPort := dotenv.GetDotEnv("FE_PORT")
	allowedOrigin := fmt.Sprintf("%s:%s", frontendHost, frontendPort)
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Upgrade", "Connection"},
		AllowCredentials: true,
	})
	return c
}
