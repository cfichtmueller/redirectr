package util

const (
	DefaultIdLength = 10
	idAlphabet      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	etagAlphabet    = "abcdef0123456789"
)

func RandomId() string {
	return RandomStringFromAlphabet(idAlphabet, DefaultIdLength)
}

func NewId(length int) string {
	return RandomStringFromAlphabet(idAlphabet, length)
}

func NewEtag() string {
	return RandomStringFromAlphabet(etagAlphabet, 64)
}
