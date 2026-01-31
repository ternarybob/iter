// Project Beta - Calculator Service
// This project provides mathematical calculation functionality
package main

import (
	"fmt"
	"math"
)

// BetaCalculator performs mathematical operations for Project Beta
type BetaCalculator struct {
	precision int
	history   []float64
}

// NewBetaCalculator creates a new calculator with specified precision
func NewBetaCalculator(precision int) *BetaCalculator {
	return &BetaCalculator{
		precision: precision,
		history:   make([]float64, 0),
	}
}

// Add performs addition and stores result in history
func (c *BetaCalculator) Add(a, b float64) float64 {
	result := a + b
	c.history = append(c.history, result)
	return result
}

// Multiply performs multiplication and stores result in history
func (c *BetaCalculator) Multiply(a, b float64) float64 {
	result := a * b
	c.history = append(c.history, result)
	return result
}

// SquareRoot calculates the square root of a number
func (c *BetaCalculator) SquareRoot(n float64) float64 {
	result := math.Sqrt(n)
	c.history = append(c.history, result)
	return result
}

// GetHistory returns all calculation results
func (c *BetaCalculator) GetHistory() []float64 {
	return c.history
}

// BetaConfig holds configuration for the Beta service
type BetaConfig struct {
	ServiceName   string
	MaxPrecision  int
	EnableLogging bool
}

// DefaultBetaConfig returns the default configuration
func DefaultBetaConfig() BetaConfig {
	return BetaConfig{
		ServiceName:   "beta-calculator-service",
		MaxPrecision:  10,
		EnableLogging: true,
	}
}

// ComputeAverage calculates the average of a slice of numbers
func ComputeAverage(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}
	sum := 0.0
	for _, n := range numbers {
		sum += n
	}
	return sum / float64(len(numbers))
}

func main() {
	calc := NewBetaCalculator(2)
	config := DefaultBetaConfig()

	fmt.Printf("Starting %s with precision %d\n", config.ServiceName, config.MaxPrecision)

	result := calc.Add(10, 20)
	fmt.Printf("10 + 20 = %.2f\n", result)

	result = calc.Multiply(5, 6)
	fmt.Printf("5 * 6 = %.2f\n", result)

	result = calc.SquareRoot(144)
	fmt.Printf("sqrt(144) = %.2f\n", result)

	avg := ComputeAverage(calc.GetHistory())
	fmt.Printf("Average of results: %.2f\n", avg)
}
