//go:build windows

package bitz

import (
	"encoding/base64"
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

func protect(value string) (string, error) {
	in := []byte(value)
	if len(in) == 0 {
		return "", errors.New("segredo vazio")
	}
	input := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	data := unsafe.Slice(output.Data, output.Size)
	return base64.StdEncoding.EncodeToString(data), nil
}
func unprotect(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil || len(raw) == 0 {
		return "", errors.New("credencial Bitz protegida inválida")
	}
	input := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var output windows.DataBlob
	if err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return string(unsafe.Slice(output.Data, output.Size)), nil
}
