package helpers

import (
	"math/rand"
	"strconv"
	"time"

	pkg "github.com/mudler/luet/pkg/package"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func String(length int) string {
	return StringWithCharset(length, charset)
}

func RandomPackage() pkg.Package {
	return pkg.NewPackage(String(5), strconv.Itoa(rand.Intn(100)), []*pkg.DefaultPackage{}, []*pkg.DefaultPackage{})
}
