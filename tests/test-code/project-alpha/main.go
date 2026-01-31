// Project Alpha - Greeting Service
// This project provides greeting functionality with customizable messages
package main

import (
	"fmt"
	"strings"
)

// AlphaGreeter handles greeting operations for Project Alpha
type AlphaGreeter struct {
	prefix string
	suffix string
}

// NewAlphaGreeter creates a new greeter with default settings
func NewAlphaGreeter() *AlphaGreeter {
	return &AlphaGreeter{
		prefix: "Hello from Alpha",
		suffix: "Have a great day!",
	}
}

// Greet generates a personalized greeting message
func (g *AlphaGreeter) Greet(name string) string {
	return fmt.Sprintf("%s, %s! %s", g.prefix, name, g.suffix)
}

// GreetMultiple greets multiple people at once
func (g *AlphaGreeter) GreetMultiple(names []string) []string {
	greetings := make([]string, len(names))
	for i, name := range names {
		greetings[i] = g.Greet(name)
	}
	return greetings
}

// AlphaConfig holds configuration for the Alpha service
type AlphaConfig struct {
	ServiceName string
	Port        int
	Debug       bool
}

// DefaultAlphaConfig returns the default configuration
func DefaultAlphaConfig() AlphaConfig {
	return AlphaConfig{
		ServiceName: "alpha-greeting-service",
		Port:        8080,
		Debug:       false,
	}
}

// FormatGreeting formats the greeting with uppercase transformation
func FormatGreeting(greeting string) string {
	return strings.ToUpper(greeting)
}

func main() {
	greeter := NewAlphaGreeter()
	config := DefaultAlphaConfig()

	fmt.Printf("Starting %s on port %d\n", config.ServiceName, config.Port)

	message := greeter.Greet("World")
	fmt.Println(message)

	formatted := FormatGreeting(message)
	fmt.Println("Formatted:", formatted)
}
