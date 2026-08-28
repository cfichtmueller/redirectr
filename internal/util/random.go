package util

import (
	"crypto/rand"
	"fmt"
	"strings"
)

func RandomStringFromAlphabet(alphabet string, length int) string {
	result := strings.Builder{}
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(fmt.Errorf("couldn't create random: %v", err))
	}
	chars := strings.Split(alphabet, "")
	cap := len(alphabet) - 1
	for i := 0; i < length; i++ {
		index := int(bytes[i]) % cap
		result.WriteString(chars[index])
	}
	return result.String()
}
