package tests

import (
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
)

func GetEnvOrSkip(env string) string {
	value := os.Getenv(env)
	if value == "" {
		ginkgo.Skip(fmt.Sprintf("%s not exported", env))
	}

	return value
}
