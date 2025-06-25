package main

import (
	"fmt"
	"log"
	"os"

	"github.com/thromel/go-database/pkg/api"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Println("Go Database Engine v0.1.0 (Sprint 1)")
		fmt.Println("Core infrastructure and basic storage implemented")
	case "demo":
		runDemo()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Go Database Engine CLI")
	fmt.Println()
	fmt.Println("Usage: go-database <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  demo       Run a simple demonstration")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Note: Full CLI functionality will be implemented in future sprints.")
}

func runDemo() {
	fmt.Println("Go Database Engine Demo")
	fmt.Println("=======================")

	// Create a temporary database
	config := api.DefaultConfig()
	config.Path = "demo.db"

	db, err := api.Open("demo.db", config)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Warning: Failed to close database: %v", err)
		}
	}()

	fmt.Println("✓ Database opened successfully")

	// Store some demo data
	demoData := map[string]string{
		"user:1":      "alice@example.com",
		"user:2":      "bob@example.com",
		"config:port": "8080",
		"config:host": "localhost",
	}

	fmt.Println("✓ Storing demo data...")
	for key, value := range demoData {
		err = db.Put([]byte(key), []byte(value))
		if err != nil {
			log.Fatalf("Failed to store %s: %v", key, err)
		}
		fmt.Printf("  - Stored: %s = %s\n", key, value)
	}

	// Retrieve and display data
	fmt.Println("✓ Retrieving data...")
	for key := range demoData {
		value, err := db.Get([]byte(key))
		if err != nil {
			log.Fatalf("Failed to get %s: %v", key, err)
		}
		fmt.Printf("  - Retrieved: %s = %s\n", key, string(value))
	}

	// Show statistics
	stats, err := db.Stats()
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Println("✓ Database statistics:")
	fmt.Printf("  - Total keys: %d\n", stats.KeyCount)
	fmt.Printf("  - Data size: %d bytes\n", stats.DataSize)

	// Test deletion
	fmt.Println("✓ Testing deletion...")
	err = db.Delete([]byte("user:1"))
	if err != nil {
		log.Fatalf("Failed to delete user:1: %v", err)
	}
	fmt.Println("  - Deleted: user:1")

	// Verify deletion
	exists, err := db.Exists([]byte("user:1"))
	if err != nil {
		log.Fatalf("Failed to check existence: %v", err)
	}
	fmt.Printf("  - user:1 exists: %v\n", exists)

	fmt.Println()
	fmt.Println("✓ Demo completed successfully!")
	fmt.Println("  All basic operations working correctly.")
}
