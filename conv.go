package rdflibgo

import "strconv"

func intToString(v int) string {
	return strconv.Itoa(v)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func float32ToString(v float32) string {
	return strconv.FormatFloat(float64(v), 'g', -1, 32)
}

func float64ToString(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

