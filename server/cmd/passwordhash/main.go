// passwordhash reads a password from stdin and emits only its salted hash.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
)

func main() {
	b, err := io.ReadAll(io.LimitReader(os.Stdin, 1026))
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read password")
		os.Exit(1)
	}
	hash, err := auth.HashPassword(strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
