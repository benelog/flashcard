package main

import "fmt"

const passMark = 0.8

func accuracy(results []bool) (float64, int) {
	correct := 0
	for _, ok := range results {
		if ok {
			correct++
		}
	}
	if len(results) == 0 {
		return 0, 0
	}
	return float64(correct) / float64(len(results)), correct
}

func grade(rate float64) string {
	switch {
	case rate >= passMark:
		return "잘함"
	case rate >= 0.5:
		return "보통"
	default:
		return "다시"
	}
}

func main() {
	results := []bool{true, true, false, true, true, false, true}
	rate, correct := accuracy(results)
	fmt.Printf("%d장 중 %d장 정답, 정답률 %.0f%%\n", len(results), correct, rate*100)
	fmt.Println("평가:", grade(rate))
}
