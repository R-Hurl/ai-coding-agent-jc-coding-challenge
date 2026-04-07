package utils

import "fmt"

func FormatOutput(operation string, a, b, result float64) string {
	return fmt.Sprintf("%s(%.2f, %.2f) = %.2f", operation, a, b, result)
}
