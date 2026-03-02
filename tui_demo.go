package main

import (
	"fmt"
	"github.com/johnjansen/anvil/internal/tui"
)

func main() {
	fmt.Println("TUI package imported successfully")
	// This is just a simple test to verify the package can be imported
	// The actual implementation would be tested through the ps command
	tui.NewModel(1234, 4)
	fmt.Printf("Created model with PID %d and %d workers\n", 1234, 4)
}