package ui

import (
	"math"
	"strings"
)

func Gradient(value float64, maximum float64, length int) string {
	length -= 2
	left := min(length, int(math.Ceil(float64(length)*value/maximum)))
	right := length - left

	var output strings.Builder
	output.WriteRune('|')
	if left > 0 {
		output.WriteString(strings.Repeat("*", left))
	}
	if right > 0 {
		output.WriteString(strings.Repeat("-", right))
	}
	output.WriteRune('|')
	return output.String()
}
