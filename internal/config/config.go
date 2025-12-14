package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	DBUrl    string
	Port     string
	RedisUrl string
}

func init() {
	// Find the project root by searching for go.mod file
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		log.Fatal("Could not find project root (go.mod file) to load .env")
	}

	envPath := filepath.Join(projectRoot, ".env")

	// Load the .env file from the determined path
	if err := godotenv.Load(envPath); err != nil {

		log.Fatalf("Error loading .env file from %s: %v", envPath, err)
	}
}

// LoadConfig loads environment variables from .env file
func LoadConfig() (*Config, error) {
	// Load .env file if exists
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	config := &Config{
		DBUrl:    os.Getenv("DB_URL"),
		Port:     os.Getenv("PORT"),
		RedisUrl: os.Getenv("REDIS_URL"),
	}

	return config, nil
}

// Helper function to find the project root by locating the go.mod file
func findProjectRoot() string {
	// Start searching from the directory where this source file lives
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	currentDir := filepath.Dir(filename)

	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir // Found the root!
		}
		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root, go.mod not found
			return ""
		}
		currentDir = parentDir
	}
}
