//go:build js && wasm

package periwiki

func init() { ContentFS = embeddedFS }
