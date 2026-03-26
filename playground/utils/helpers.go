package utils

import "fmt"

func FormatResult(operation string, a, b, result float64) string {
	return fmt.Sprintf("%s(%.2f, %.2f) = %.2f", operation, a, b, result)
}
