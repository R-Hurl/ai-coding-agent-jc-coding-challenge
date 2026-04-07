package main

import (
	"fmt"

	"github.com/ryan/ai-coding-agent/playground/calculator"
	"github.com/ryan/ai-coding-agent/playground/utils"
)

func main() {
	a, b := 12.0, 4.0

	fmt.Println(utils.FormatOutput("Add", a, b, calculator.Add(a, b)))
	fmt.Println(utils.FormatOutput("Subtract", a, b, calculator.Subtract(a, b)))
	fmt.Println(utils.FormatOutput("Multiply", a, b, calculator.Multiply(a, b)))

	result, err := calculator.Divide(a, b)
	if err != nil {
		fmt.Println("Divide error:", err)
	} else {
		fmt.Println(utils.FormatOutput("Divide", a, b, result))
	}

	// Test divide by zero error handling
	_, err = calculator.Divide(a, 0)
	if err != nil {
		fmt.Println("Divide error:", err)
	}
}
