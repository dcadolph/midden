//go:build tools

package mobile

// gomobile bind requires golang.org/x/mobile/bind to be a module dependency
// even though no midden source imports it directly. This blank import keeps
// it in go.mod through go mod tidy.
import _ "golang.org/x/mobile/bind"
