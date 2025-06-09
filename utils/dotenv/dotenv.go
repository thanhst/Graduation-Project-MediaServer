package dotenv

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func GetDotEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		err := godotenv.Load("cmd/config/.env")
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		val = os.Getenv(key)
	}
	return val
}
func SetDotEnv(key, value string) error {
	envFile := "cmd/config/.env"
	input, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	output := strings.Join(lines, "\n")
	return os.WriteFile(envFile, []byte(output), 0644)
}
